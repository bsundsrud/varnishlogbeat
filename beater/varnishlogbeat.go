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
	config  *vago.Config
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
		config: &vago.Config{
			Path:    c.Path,
			Timeout: c.Timeout,
		},
	}
	return bt, nil
}

// Run starts varnishlogbeat.
func (bt *Varnishlogbeat) Run(b *beat.Beat) error {
	logp.Info("varnishlogbeat is running! Hit CTRL-C to stop it.")

	var err error

	bt.varnish, err = vago.Open(bt.config)
	if err != nil {
		return err
	}

	bt.client, err = b.Publisher.Connect()
	if err != nil {
		return err
	}

	err = bt.harvest()
	if err != nil {
		logp.Err("%s", err)
	}
	return err
}

// harvest reads and parses Varnish log data.
func (bt *Varnishlogbeat) harvest() error {
	tx := make(common.MapStr)
	counter := 1

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
			case "BereqHeader",
				"BerespHeader",
				"ObjHeader",
				"ReqHeader",
				"RespHeader",
				"Timestamp":
				header := strings.SplitN(data, ": ", 2)
				key := header[0]
				var value interface{}
				switch {
				case key == "Content-Length":
					value, _ = strconv.Atoi(header[1])
				case len(header) == 2:
					value = header[1]
				// if the header is too long, header and value might get truncated
				default:
					value = "truncated"
				}
				if _, ok := tx[tag]; ok {
					tx[tag].(common.MapStr)[key] = value
				} else {
					tx[tag] = common.MapStr{key: value}
				}

			case "Length":
				tx[tag], _ = strconv.Atoi(data)

			case "End":
				fields := common.MapStr{
					"count": counter,
					"type":  _type,
					"vxid":  vxid,
					"tx":    tx,
				}
				event := beat.Event{
					Timestamp: time.Now(),
					Fields:    fields,
				}
				bt.client.Publish(event)
				counter++
				logp.Info("Event sent")

				// destroy and re-create the map
				tx = nil
				tx = make(common.MapStr)
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
