package control

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/emicklei/melrose/core"
	"github.com/emicklei/melrose/notify"
)

type Listen struct {
	mutex           *sync.RWMutex
	ctx             core.Context
	deviceID        int
	variableStore   core.VariableStorage
	variableName    string
	isRunning       bool
	callback        core.Valueable
	notesOn         map[int]int
	noteChangeCount int
}

func NewListen(ctx core.Context, deviceID int, variableName string, target core.Valueable) *Listen {
	return &Listen{
		mutex:           new(sync.RWMutex),
		ctx:             ctx,
		deviceID:        deviceID,
		variableName:    variableName,
		callback:        target,
		notesOn:         map[int]int{},
		noteChangeCount: 0,
	}
}

// Inspect implements Inspectable
func (l *Listen) Inspect(i core.Inspection) {
	i.Properties["running"] = l.isRunning
}

// Target is for replacing functions
func (l *Listen) Target() core.Valueable { return l.callback }

// SetTarget is for replacing functions
func (l *Listen) SetTarget(c core.Valueable) { l.callback = c }

// Play is part of core.Playable
func (l *Listen) Play(ctx core.Context, at time.Time) error {
	if l.isRunning {
		return nil
	}
	if !ctx.Device().HasInputCapability() {
		return errors.New("Input in not available for this device")
	}
	l.isRunning = true
	ctx.Device().Listen(l.deviceID, l, l.isRunning)
	return nil
}

func (l *Listen) Stop(ctx core.Context) error {
	if !l.isRunning {
		return nil
	}
	l.isRunning = false
	ctx.Device().Listen(l.deviceID, l, l.isRunning)
	return nil
}

// NoteOn is part of core.NoteListener
func (l *Listen) NoteOn(n core.Note) {
	l.mutex.Lock()
	if core.IsDebug() {
		notify.Debugf("control.listen ON %v", n)
	}
	l.noteChangeCount++
	countCheck := l.noteChangeCount
	nr := n.MIDI()
	l.notesOn[nr] = countCheck
	l.ctx.Variables().Put(l.variableName, n)

	// release so condition can be evaluated
	l.mutex.Unlock()

	if e, ok := l.callback.Value().(core.Evaluatable); ok {
		// only actually play the note if the hit count matches the check
		cond := func() bool {
			return l.isNoteOnCount(nr, countCheck)
		}
		e.Evaluate(l.ctx.WithCondition(cond))
	}
}

func (l *Listen) isNoteOnCount(nr, countCheck int) bool {
	l.mutex.RLock()
	defer l.mutex.RUnlock()
	// is the note still on?
	count, ok := l.notesOn[nr]
	// is the note on on the count
	return ok && count == countCheck
}

// NoteOff is part of core.NoteListener
func (l *Listen) NoteOff(n core.Note) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	if core.IsDebug() {
		notify.Debugf("control.listen OFF %v", n)
	}
	delete(l.notesOn, n.MIDI())
}

// Storex is part of core.Storable
func (l *Listen) Storex() string {
	return fmt.Sprintf("listen(%d,%s,%s)", l.deviceID, l.variableName, core.Storex(l.callback))
}
