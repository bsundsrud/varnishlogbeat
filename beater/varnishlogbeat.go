package beater

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/phenomenes/vago"

	"github.com/bsundsrud/varnishlogbeat/config"
)

// Varnishlogbeat configuration.
type Varnishlogbeat struct {
	config  *config.Config
	client  beat.Client
	varnish *vago.Varnish
}

// New creates an instance of varnishlogbeat.
func New(b *beat.Beat, cfg *common.Config) (beat.Beater, error) {
	c := config.DefaultConfig
	if err := cfg.Unpack(&c); err != nil {
		return nil, fmt.Errorf("Error reading config file: %v", err)
	}

	bt := &Varnishlogbeat{
		config: &c,
	}
	return bt, nil
}

// Run starts varnishlogbeat.
func (bt *Varnishlogbeat) Run(b *beat.Beat) error {
	logp.Info("varnishlogbeat is running! Hit CTRL-C to stop it.")

	var err error

	bt.varnish, err = vago.Open(&vago.Config{
		Path:    bt.config.Path,
		Timeout: bt.config.Timeout,
	})
	if err != nil {
		return err
	}
	logp.Info("Connected to varnish at path %s", bt.config.Path)
	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}
	logp.Info("Connected to publisher.")
	err = bt.harvest()
	if err != nil {
		logp.Err("%s", err)
	}
	return err
}

func parseMapString(data string) (string, string) {
	header := strings.SplitN(data, ": ", 2)
	key := header[0]
	var value string
	switch {
	case len(header) == 2:
		value = header[1]
		// if the header is too long, header and value might get truncated
	default:
		value = "truncated"
	}
	return key, value
}

func headerIncluded(whitelist []string, header string) bool {
	for _, hdr := range whitelist {
		if hdr == header {
			return true
		}
	}
	return false
}

func extractDurationMs(timestamp string) float64 {
	parts := strings.SplitN(timestamp, " ", 3)
	if len(parts) != 3 {
		return -1
	}
	val, err := strconv.ParseFloat(parts[1], 64)
	if err != nil {
		return -1
	}
	return val * 1000
}

// harvest reads and parses Varnish log data.
func (bt *Varnishlogbeat) harvest() error {
	tx := make(common.MapStr)
	counter := 1
	var durationMs float64 = -1
	var ttfbMs float64 = -1
	startOnly := true
	var startInitialDurationMs float64 = -1
	var parentVxid uint32 = 0
	subtype := ""
	logp.Info("Starting harvesting")
	bt.varnish.Log("",
		vago.REQ,
		vago.COPT_TAIL|vago.COPT_BATCH,
		func(vxid uint32, tag, _type, data string) int {
			switch _type {
			case "c":
				_type = "client"
			case "b":
				_type = "backend"
			default:
				return 0
			}

			switch tag {
			case "Begin":
				tx[tag] = data
				parts := strings.SplitN(data, " ", 3)
				if len(parts) == 3 {
					val, _ := strconv.ParseUint(parts[1], 10, 32)
					parentVxid = uint32(val)
					subtype = parts[2]
				}
			case "ReqHeader", "BereqHeader":
				key, strValue := parseMapString(data)
				if !headerIncluded(bt.config.IncludeHeaders.ReqHeaders, key) {
					return 0
				}
				var value interface{}
				if key == "Content-Length" {
					value, _ = strconv.Atoi(strValue)
				} else {
					value = strValue
				}
				if _, ok := tx[tag]; ok {
					tx[tag].(common.MapStr)[key] = value
				} else {
					tx[tag] = common.MapStr{key: value}
				}
			case "RespHeader", "BerespHeader":
				key, strValue := parseMapString(data)
				if !headerIncluded(bt.config.IncludeHeaders.RespHeaders, key) {
					return 0
				}
				var value interface{}
				if key == "Content-Length" {
					value, _ = strconv.Atoi(strValue)
				} else {
					value = strValue
				}
				if _, ok := tx[tag]; ok {
					tx[tag].(common.MapStr)[key] = value
				} else {
					tx[tag] = common.MapStr{key: value}
				}
			case "ObjHeader":
				key, strValue := parseMapString(data)
				if !headerIncluded(bt.config.IncludeHeaders.ObjHeaders, key) {
					return 0
				}
				var value interface{}
				if key == "Content-Length" {
					value, _ = strconv.Atoi(strValue)
				} else {
					value = strValue
				}
				if _, ok := tx[tag]; ok {
					tx[tag].(common.MapStr)[key] = value
				} else {
					tx[tag] = common.MapStr{key: value}
				}
			case "Hit":
				tx["HitRaw"] = data

				var objVxid uint64
				var remainingTtl float64
				var gracePeriod float64
				var keepPeriod float64
				parts := strings.SplitN(data, " ", 4)
				switch len(parts) {
				case 4:
					keepPeriod, _ = strconv.ParseFloat(parts[3], 64)
					fallthrough
				case 3:
					gracePeriod, _ = strconv.ParseFloat(parts[2], 64)
					fallthrough
				case 2:
					remainingTtl, _ = strconv.ParseFloat(parts[1], 64)
					fallthrough
				case 1:
					objVxid, _ = strconv.ParseUint(parts[0], 10, 32)
				}
				tx[tag] = common.MapStr{
					"ObjVxid":      uint32(objVxid),
					"RemainingTTL": remainingTtl,
					"GracePeriod":  gracePeriod,
					"KeepPeriod":   keepPeriod,
				}
			case "ReqAcct", "BereqAcct":
				parts := strings.SplitN(data, " ", 6)
				if len(parts) != 6 {
					return 0
				}
				headerRecv, _ := strconv.Atoi(parts[0])
				bodyRecv, _ := strconv.Atoi(parts[1])
				totalRecv, _ := strconv.Atoi(parts[2])
				headerTx, _ := strconv.Atoi(parts[3])
				bodyTx, _ := strconv.Atoi(parts[4])
				totalTx, _ := strconv.Atoi(parts[5])
				byteAccounting := common.MapStr{
					"HeaderRecv": headerRecv,
					"BodyRecv":   bodyRecv,
					"TotalRecv":  totalRecv,
					"HeaderTx":   headerTx,
					"BodyTx":     bodyTx,
					"TotalTx":    totalTx,
				}
				tx[tag] = byteAccounting
			case "Timestamp":
				key, value := parseMapString(data)
				if _, ok := tx[tag]; ok {
					tx[tag].(common.MapStr)[key] = value
				} else {
					tx[tag] = common.MapStr{key: value}
				}
				switch key {
				case "Start":
					startInitialDurationMs = extractDurationMs(value)
				case "Resp", "BerespBody", "Error", "Retry", "PipeSess", "Restart":
					durationMs = extractDurationMs(value)
					startOnly = false
				case "Process", "Beresp":
					ttfbMs = extractDurationMs(value)
					startOnly = false
				default:
					startOnly = false
				}

			case "Length":
				tx[tag], _ = strconv.Atoi(data)

			case "End":
				if startOnly && startInitialDurationMs >= 0 {
					durationMs = startInitialDurationMs
				}
				if durationMs >= 0 {
					tx["DurationMs"] = durationMs
				}
				if ttfbMs >= 0 {
					tx["firstByteMs"] = ttfbMs
				}
				fields := common.MapStr{
					"count":       counter,
					"type":        _type,
					"subtype":     subtype,
					"vxid":        vxid,
					"parent_vxid": parentVxid,
					"tx":          tx,
				}
				event := beat.Event{
					Timestamp: time.Now(),
					Fields:    fields,
				}
				bt.client.Publish(event)
				counter++

				// destroy and re-create the map, general cleanup
				tx = nil
				tx = make(common.MapStr)
				startOnly = true
				startInitialDurationMs = -1
				ttfbMs = -1
				durationMs = -1
				parentVxid = 0
				subtype = ""
			default:
				tx[tag] = data
			}

			return 0
		})

	return nil
}

// Stop stops varnishlogbeat.
func (bt *Varnishlogbeat) Stop() {
	bt.varnish.Stop()
	bt.varnish.Close()
	bt.client.Close()
}
