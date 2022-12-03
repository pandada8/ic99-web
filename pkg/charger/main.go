package charger

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"time"

	"go.bug.st/serial"
)

var (
	CHANNEL_INDEX_NORMAL     = []int{3, 4, 1, 2}
	CHANNEL_INDEX_MODE       = revmap([]int{4, 3, 1, 2}) // 通道模式按照 4,3,1,2 的顺序排列
	CHANNEL_INDEX_STATUS_1_3 = revmap([]int{2, 1, 3, 4}) // 通道状态按照
)

func revmap(arr []int) (ret []int) {
	m := make(map[int]int)
	for i := range arr {
		m[arr[i]-1] = i + 1
	}
	ret = make([]int, len(arr))
	for k, v := range m {
		ret[k] = v
	}
	return
}

func StartLoop() {
	charger := Charger{SerialPort: "/dev/ttyUSB0", ID: "1"}
	charger.Start()
}

type Charger struct {
	SerialPort string
	port       serial.Port
	ID         string
}

func (c *Charger) Start() (err error) {
	mode := &serial.Mode{
		BaudRate: 9600,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
	}
	c.port, err = serial.Open(c.SerialPort, mode)
	if err != nil {
		return err
	}
	go c.readData()
	return nil
}

func (c *Charger) readData() {
	var buf []byte
	reader := bufio.NewReader(c.port)
	log.Printf("start reading %s", c.SerialPort)
	for {
		for {
			read, err := reader.ReadBytes(0xAA)
			if err != nil {
				log.Println(err)
				return
			}
			buf = append(buf, read...)

			if len(buf) >= 74 && buf[len(buf)-74] == 0xFF && buf[len(buf)-73] == 0xFE {
				buf = buf[len(buf)-74:]
				break
			}
		}
		// FIXME: using proper logging debug
		log.Println("read", hex.EncodeToString(buf))
		data, err := c.parseData(buf)
		if err != nil {
			c.handleError(err)
			return
		}
		Broadcast(data)
	}
}

func (c *Charger) parseData(buf []byte) (cd ChargerData, err error) {
	// assert header
	if buf[0] != 0xFF || buf[1] != 0xFE || buf[73] != 0xAA {
		err = errors.New("invalid packet")
		return
	}
	// log.Printf("buf[6]: %b", buf[6])
	// log.Printf("buf[7]: %b", buf[7])
	// log.Printf("buf[8]: %b", buf[8])
	for index := 0; index < 4; index++ {
		ch := ChargerChannel{Index: index}
		ch.parseModeAndStatus(buf)
		ch.parseCurrentAndVoltage(buf)
		cd.Channels = append(cd.Channels, ch)
	}
	cd.ID = c.ID
	return
}

func (c *Charger) handleError(err error) {
	log.Println(err)
}

type ChargerData struct {
	Channels []ChargerChannel `json:"channels"`
	ID       string
}

type ChargerChannel struct {
	Index             int
	Mode              ChargerMode
	Status            ChargerStatus
	ConfiguredCurrent uint16
	Current           uint16
	OfflineVoltage    uint16
	OnlineVoltage     uint16
	ChargeCapacity    uint32
	DischargeCapacity uint32
	Time              uint16
	Impedance         uint16
}

func (c *ChargerChannel) parseModeAndStatus(buf []byte) {
	c.Mode = ChargerMode(buf[2+CHANNEL_INDEX_MODE[c.Index]-1])

	offset_1_3 := 2 * (4 - CHANNEL_INDEX_STATUS_1_3[c.Index])
	offset_2 := c.Index
	// log.Println(c.Index, offset_1_3, offset_2, buf[6]>>offset_1_3, buf[7]>>offset_2)
	if (buf[6]>>offset_1_3)&0x01 == 0x01 {
		c.Status = STATUS_CHARGE
	} else if (buf[6]>>offset_1_3)&0x02 == 0x02 {
		c.Status = STATUS_DISCHARGE
	} else if (buf[7]>>offset_2)&0x01 == 0x01 {
		c.Status = STATUS_EMPTY
	} else if (buf[7]>>(offset_2+4))&0x01 == 0x01 {
		c.Status = STATUS_COMPLETE
	} else if buf[8]&0x10 == 0x10 {
		c.Status = STATUS_TEMPROTECT
	} else if (buf[8]>>(offset_1_3/2))&0x01 == 0x01 {
		c.Status = STATUS_REPAUSE
	}
}

func (c *ChargerChannel) parseCurrentAndVoltage(buf []byte) {
	start := CHANNEL_INDEX_MODE[c.Index] - 1
	end := start + 1

	c.ConfiguredCurrent = binary.LittleEndian.Uint16(buf[9+start*2 : 9+end*2])
	if c.Status != STATUS_EMPTY {
		c.Current = binary.LittleEndian.Uint16(buf[17+start*2 : 17+end*2])
		c.OfflineVoltage = binary.LittleEndian.Uint16(buf[25+start*2 : 25+end*2])
		c.OnlineVoltage = binary.LittleEndian.Uint16(buf[33+start*2 : 33+end*2])
		if c.Status == STATUS_CHARGE {
			c.ChargeCapacity = binary.LittleEndian.Uint32(buf[41+start*4 : 41+end*4])
		} else {
			c.DischargeCapacity = binary.LittleEndian.Uint32(buf[41+start*4 : 41+end*4])
		}
		c.Impedance = binary.LittleEndian.Uint16(buf[65+start*2 : 65+end*2])
		c.Time = binary.BigEndian.Uint16(buf[57+start*2 : 57+end*2])
	}
}

func (c ChargerChannel) GetChargeCapacity() float64 {
	return float64(c.ChargeCapacity) / 4096
}

func (c ChargerChannel) GetDischargeCapacity() float64 {
	return float64(c.DischargeCapacity) / 4096
}

func (c ChargerChannel) GetDuration() time.Duration {
	return time.Duration(c.Time) * time.Minute
}

func (c ChargerChannel) MarshalJSON() ([]byte, error) {
	ret := map[string]interface{}{
		"index":              c.Index,
		"mode":               c.Mode,
		"status":             c.Status,
		"configured_current": c.ConfiguredCurrent,
		"now_current":        c.Current,
		"offline_voltage":    c.OfflineVoltage,
		"online_voltage":     c.OnlineVoltage,
		"charge_capacity":    c.GetChargeCapacity(),
		"discharge_capacity": c.GetDischargeCapacity(),
		"duration_min":       c.Time,
	}
	return json.Marshal(ret)
}

var ChargerModes = map[byte]string{
	0x01: "CHARGE",
	0x02: "DISCHARGE",
	0x04: "REFRESH",
	0x08: "CHARGE_TEST",
	0x10: "IMPEDANCE_TEST",
}

type ChargerMode byte

var (
	MODE_CHARGE         ChargerMode = 0x01
	MODE_DISCHARGE      ChargerMode = 0x02
	MODE_REFRESH        ChargerMode = 0x04
	MODE_CHARGE_TEST    ChargerMode = 0x08
	MODE_IMPEDANCE_TEST ChargerMode = 0x10
)

func (c ChargerMode) String() string {
	return ChargerModes[byte(c)]
}

func (c ChargerMode) MarshalJSON() ([]byte, error) {
	return []byte("\"" + c.String() + "\""), nil
}

type ChargerStatus string

var (
	STATUS_CHARGE     ChargerStatus = "CHARGE"
	STATUS_DISCHARGE  ChargerStatus = "DISCHARGE"
	STATUS_COMPLETE   ChargerStatus = "COMPLETE"
	STATUS_EMPTY      ChargerStatus = "EMPTY"
	STATUS_REPAUSE    ChargerStatus = "REPAUSE"
	STATUS_TEMPROTECT ChargerStatus = "TEMPROTECT"
	STATUS_UNKOWN     ChargerStatus = "UNKOWN"
)
