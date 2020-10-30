package godbi

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/google/gousb"
)

//const VID = 0x057E, PID = 0x3000;

const (
	product = 0x3000
	vendor  = 0x057e
)

const (
	dbimagic = "DBI0"
)
const (
	CMD_ID_EXIT       = 0
	CMD_ID_LIST_OLD   = 1 // DBI below 168
	CMD_ID_FILE_RANGE = 2
	CMD_ID_LIST       = 3 // DBI 168+

	CMD_TYPE_REQUEST  = 0
	CMD_TYPE_RESPONSE = 1
	CMD_TYPE_ACK      = 2
)

type (
	//     const buffer = await this.bufferRead(16);
	//     const magic = buffer.slice(0, 4).toString();
	//     const type = buffer.readUInt32LE(4);
	//     const id = buffer.readUInt32LE(8);
	//     const data_size = buffer.readUInt32LE(12);
	DBICmdHeader struct {
		Magic    [4]byte
		CMDType  uint32
		ID       uint32
		DataSize uint32
	}

	// The DBIBackend type is an API to an AKAI DBIBackend that is connected
	// to the host machine via USB.
	DBIBackend struct {
		// Fields for interacting with the USB connection
		context     *gousb.Context
		device      *gousb.Device
		intf        *gousb.Interface
		inEndpoint  *gousb.InEndpoint
		outEndpoint *gousb.OutEndpoint

		// Fields for managing async operations
		waitGroup *sync.WaitGroup

		// Channels for error reporting and
		logs   chan string
		errors chan error
		close  chan bool

		// Channels for various components
		//commands map[uint32]chan DBICommand
		// read_files map[string]chan uint64
		// size_files map[string]chan uint64
		files map[string]chan string // [nsp_name] = path
		// knobs  map[int]chan int
		// pads   map[int]chan int
	}
)

func parseRequestPacket(data []byte) (*DBICmdHeader, error) {
	if len(data) != 16 {
		return nil, fmt.Errorf("request packet wrong size: %d != 16", len(data))
	}

	if string(data[0:4]) != dbimagic {
		return nil, fmt.Errorf("request wrong packet")
	}

	packet := &DBICmdHeader{
		CMDType:  binary.LittleEndian.Uint32(data[4:]),
		ID:       binary.LittleEndian.Uint32(data[8:]),
		DataSize: binary.LittleEndian.Uint32(data[12:]),
	}
	copy(packet.Magic[:], data[0:4])

	return packet, nil
}

// Find attempts to locate an AKAI DBIBackend connected via USB and returns
// a new instance of the DBIBackend type, exposing an API for interacting
// with the device. Returns an error if the device cannot be found.
func Find() (*DBIBackend, error) {
	ctx := gousb.NewContext()
	ctx.Debug(0)
	devices, err := ctx.OpenDevices(findDBIBackend(product, vendor))

	if err != nil {
		return nil, fmt.Errorf("failed to open devices: %s", err.Error())
	}

	if len(devices) != 1 {
		return nil, fmt.Errorf("failed to find device with product %d and vendor %d", product, vendor)
	}

	if err := devices[0].SetAutoDetach(true); err != nil {
		return nil, fmt.Errorf("failed to set automatic kernel detatch: %s", err.Error())
	}

	for num := range devices[0].Desc.Configs {
		config, err := devices[0].Config(num)

		if err != nil {
			continue
		}

		defer config.Close()

		for _, desc := range config.Desc.Interfaces {
			var err error

			intf, err := config.Interface(desc.Number, 0)

			if err != nil {
				continue
			}
			var inEndpoint *gousb.InEndpoint
			var outEndpoint *gousb.OutEndpoint

			for _, endpointDesc := range intf.Setting.Endpoints {
				if endpointDesc.Direction == gousb.EndpointDirectionIn {
					inEndpoint, err = intf.InEndpoint(endpointDesc.Number)
					if err != nil {
						continue
					}
				}
				if endpointDesc.Direction == gousb.EndpointDirectionOut {
					outEndpoint, err = intf.OutEndpoint(endpointDesc.Number)
					if err != nil {
						continue
					}
				}
			}

			mpd := &DBIBackend{
				context:     ctx,
				device:      devices[0],
				waitGroup:   &sync.WaitGroup{},
				intf:        intf,
				inEndpoint:  inEndpoint,
				outEndpoint: outEndpoint,
				errors:      make(chan error),
				close:       make(chan bool),
				// commands:  make(map[uint32]chan DBICommand),
				// read_files: make(map[string]chan uint64),
				// size_files: make(map[string]chan uint64),
				files: make(map[string]chan string),
			}

			go mpd.read()
			return mpd, nil
		}
	}

	return nil, fmt.Errorf("failed to obtain configuration for device")
}

// Close stops the connection with the AKAI DBIBackend
func (mpd *DBIBackend) Close() {
	mpd.close <- true
	mpd.waitGroup.Wait()

	mpd.inEndpoint = nil
	mpd.intf.Close()
	mpd.device.Close()
	mpd.context.Close()
}

// Errors exposes a channel that returns any errors that occur
// in the underlying operations of the API and the USB device.
func (mpd *DBIBackend) Errors() <-chan error {
	return mpd.errors
}

func (mpd *DBIBackend) read() {
	for {
		select {
		case <-mpd.close:
			return
		default:
			buff := make([]byte, 16)
			n, err := mpd.inEndpoint.Read(buff)

			if err != nil {
				mpd.errors <- err
				continue
			}

			header, err := parseRequestPacket(buff[:n])

			if err != nil {
				mpd.errors <- err
				continue
			}

			mpd.handlePacket(*header)
		}
	}
}

func (mpd *DBIBackend) handleCmdList(header DBICmdHeader) {
	// defer mpd.waitGroup.Done()

	const testStr = "Animal Crossing New Horizons [01006F8002326000][v0].nsp\n"

	dataOut := DBICmdHeader{
		CMDType:  CMD_TYPE_RESPONSE,
		ID:       header.ID,
		DataSize: uint32(len(testStr)),
	}
	copy(dataOut.Magic[:], []byte(dbimagic))

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.LittleEndian, &dataOut)
	if err != nil {
		fmt.Println(err)
		return
	}

	_, err = mpd.outEndpoint.Write(buf.Bytes())

	if err != nil {
		mpd.errors <- err
	}

	askBuf := make([]byte, 16)
	n, err := mpd.inEndpoint.Read(askBuf)

	if err != nil {
		mpd.errors <- err
	}

	askHeader, err := parseRequestPacket(askBuf[:n])

	if err != nil {
		mpd.errors <- err
	}

	if askHeader.DataSize != uint32(len(testStr)) {
		mpd.errors <- fmt.Errorf("ask packet datasize mismatch: ask %d != want %d", askHeader.DataSize, len(testStr))
	}

	_, err = mpd.outEndpoint.Write([]byte(testStr))

	if err != nil {
		mpd.errors <- err
	}
}

func (mpd *DBIBackend) handlePacket(header DBICmdHeader) {
	switch header.CMDType {
	case CMD_TYPE_REQUEST:
		switch header.ID {
		case CMD_ID_LIST_OLD, CMD_ID_LIST:
			mpd.handleCmdList(header)
		}
	}
}

func findDBIBackend(product, vendor uint16) func(desc *gousb.DeviceDesc) bool {
	return func(desc *gousb.DeviceDesc) bool {
		// Find all devices whose product and vendor codes match that
		// of the AKAI DBIBackend
		return desc.Product == gousb.ID(product) && desc.Vendor == gousb.ID(vendor)
	}
}
