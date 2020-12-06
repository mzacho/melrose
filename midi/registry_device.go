package midi

import (
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/emicklei/melrose/core"
	"github.com/emicklei/melrose/notify"
	"github.com/emicklei/tre"
	"github.com/rakyll/portmidi"
)

type DeviceRegistry struct {
	mutex           *sync.RWMutex
	in              map[int]*InputDevice
	out             map[int]*OutputDevice
	defaultInputID  int
	defaultOutputID int
	streamRegistry  *streamRegistry
}

func NewDeviceRegistry() (*DeviceRegistry, error) {
	r := &DeviceRegistry{
		mutex:          new(sync.RWMutex),
		in:             map[int]*InputDevice{},
		out:            map[int]*OutputDevice{},
		streamRegistry: newStreamRegistry(),
	}
	if err := r.init(); err != nil {
		return nil, err
	}
	return r, nil
}

func (d *DeviceRegistry) IO() (inputDeviceID, outputDeviceID int) {
	return d.defaultInputID, d.defaultOutputID
}

func (d *DeviceRegistry) Reset() {
	for _, each := range d.out {
		each.Reset()
	}
	for _, each := range d.in {
		each.stopListener()
	}
}

func (d *DeviceRegistry) Output(id int) (*OutputDevice, error) {
	d.mutex.RLock()
	if m, ok := d.out[id]; ok {
		d.mutex.RUnlock()
		return m, nil
	}
	d.mutex.RUnlock()
	// not present
	d.mutex.Lock()
	defer d.mutex.Unlock()
	midiOut, err := d.streamRegistry.output(id)
	if err != nil {
		return nil, tre.New(err, "Output", "id", id)
	}
	od := NewOutputDevice(id, midiOut, 1)
	d.out[id] = od
	od.Start() // play outgoing notes
	return od, nil
}

func (d *DeviceRegistry) Input(id int) (*InputDevice, error) {
	d.mutex.RLock()
	if m, ok := d.in[id]; ok {
		d.mutex.RUnlock()
		return m, nil
	}
	d.mutex.RUnlock()
	// not present
	d.mutex.Lock()
	defer d.mutex.Unlock()
	midiIn, err := d.streamRegistry.input(id)
	if err != nil {
		return nil, tre.New(err, "Input", "id", id)
	}
	ide := NewInputDevice(id, midiIn)
	d.in[id] = ide
	return ide, nil
}

func (d *DeviceRegistry) init() error {
	portmidi.Initialize()
	outputID := portmidi.DefaultOutputDeviceID()
	if outputID == -1 {
		return errors.New("no default output MIDI device available")
	}
	d.defaultOutputID = int(outputID)
	return nil
}

func (d *DeviceRegistry) Close() error {
	defer portmidi.Terminate()
	for _, each := range d.in {
		each.stopListener()
	}
	return d.streamRegistry.close()
}

// Command is part of melrose.AudioDevice
func (d *DeviceRegistry) Command(args []string) notify.Message {
	if len(args) == 0 {
		d.printInfo()
		return nil
	}
	switch args[0] {
	case "echo":
		od, _ := d.Output(d.defaultOutputID)
		od.echo = !od.echo
		return notify.Infof("printing notes enabled:%v", od.echo)
	// case "channel":
	// 	if len(args) != 2 {
	// 		return notify.Warningf("missing channel number")
	// 	}
	// 	nr, err := strconv.Atoi(args[1])
	// 	if err != nil {
	// 		return notify.Errorf("bad channel number:%v", err)
	// 	}
	// 	if nr < 1 || nr > 16 {
	// 		return notify.Errorf("bad channel number; must be in [1..16]")
	// 	}
	// 	m.defaultOutputChannel = nr
	// 	return nil
	case "in":
		if len(args) != 2 {
			return notify.Warningf("missing device number")
		}
		nr, err := strconv.Atoi(args[1])
		if err != nil {
			return notify.Errorf("bad device number:%v", err)
		}
		d.defaultInputID = nr
		return notify.Infof("Current input device id:%v", nr)
	case "out":
		if len(args) != 2 {
			return notify.Warningf("missing device number")
		}
		nr, err := strconv.Atoi(args[1])
		if err != nil {
			return notify.Errorf("bad device number:%v", err)
		}
		d.defaultOutputID = nr
		return notify.Infof("Current output device id:%v", nr)
	default:
		return notify.Warningf("unknown device access command: %s", args[0])
	}
}

func (d *DeviceRegistry) printInfo() {
	fmt.Println("\033[1;33mUsage:\033[0m")
	fmt.Println(":m echo                --- toggle printing the notes that are send")
	fmt.Println(":m in      <device-id> --- change the default MIDI input  device id")
	fmt.Println(":m out     <device-id> --- change the default MIDI output device id")
	fmt.Println()

	fmt.Println("\033[1;33mAvailable:\033[0m")
	var midiDeviceInfo *portmidi.DeviceInfo
	for i := 0; i < portmidi.CountDevices(); i++ {
		midiDeviceInfo = portmidi.Info(portmidi.DeviceID(i)) // returns info about a MIDI device
		fmt.Printf("[midi] device %d: ", i)
		usage := "output"
		if midiDeviceInfo.IsInputAvailable {
			usage = "input"
		}
		oc := "open"
		if !midiDeviceInfo.IsOpened {
			oc = "closed"
		}
		fmt.Print("\"", midiDeviceInfo.Interface, "/", midiDeviceInfo.Name, "\"",
			", is ", oc, " for ", usage)
		fmt.Println()
	}
	fmt.Println()

	fmt.Println("\033[1;33mCurrent:\033[0m")

	midiDeviceInfo = portmidi.Info(portmidi.DeviceID(d.defaultInputID))
	fmt.Printf("[midi] device  %d = default  input, %s/%s\n", d.defaultInputID, midiDeviceInfo.Interface, midiDeviceInfo.Name)

	midiDeviceInfo = portmidi.Info(portmidi.DeviceID(d.defaultOutputID))
	fmt.Printf("[midi] device  %d = default output, %s/%s\n", d.defaultOutputID, midiDeviceInfo.Interface, midiDeviceInfo.Name)

	od, _ := d.Output(d.defaultOutputID)
	fmt.Printf("[midi] channel %d = default MIDI output channel\n", od.defaultChannel)
	fmt.Printf("[midi] echo notes = %v\n", od.echo)

	// debug stuff
	for deviceID, each := range d.out {
		if trace, ok := each.stream.(tracingMIDIStream); ok {
			trace.log(deviceID)
		}
	}
}

func (d *DeviceRegistry) ChangeInputDeviceID(id int) {
	d.defaultInputID = id
}
func (d *DeviceRegistry) ChangeOutputDeviceID(id int) {
	d.defaultOutputID = id
}
func (d *DeviceRegistry) EchoReceivedPitchOnly(bool) {
	// TODO
}

func (r *DeviceRegistry) Listen(deviceID int, who core.NoteListener, startOrStop bool) {
	if core.IsDebug() {
		notify.Debugf("midi.listen id=%d, start=%v", deviceID, startOrStop)
	}
	in, err := r.Input(deviceID)
	if err != nil {
		notify.Warningf("input creation failed:%v", err)
		return
	}
	if startOrStop {
		in.listener.start()
		// wait for pending events to be flushed
		time.Sleep(200 * time.Millisecond)
		in.listener.add(who)
	} else {
		in.listener.remove(who)
		// do not stop the listener such that incoming events are just ignored
	}
}
