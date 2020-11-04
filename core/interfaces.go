package core

import (
	"time"

	"github.com/emicklei/melrose/notify"
)

type Sequenceable interface {
	S() Sequence
}

type NoteConvertable interface {
	ToNote() (Note, error)
}

type Storable interface {
	Storex() string
}

type Indexable interface {
	At(i int) Sequenceable
}

type Nextable interface {
	Next() interface{}
}

type AudioDevice interface {
	// Per device specific commands
	Command(args []string) notify.Message

	// Play schedules all the notes on the timeline using a BPM (beats-per-minute).
	// Returns the end time of the last played Note.
	Play(seq Sequenceable, bpm float64, beginAt time.Time) (endingAt time.Time)

	// Schedule put an event on the timeline at a begin
	Schedule(event TimelineEvent, beginAt time.Time)

	// Record(ctx Context) (*Recording, error)
	Reset()
	Close() error
}

type LoopController interface {
	Start()
	Stop()
	Reset()

	SetBPM(bpm float64)
	BPM() float64

	SetBIAB(biab int)
	BIAB() int

	StartLoop(l *Loop)
	EndLoop(l *Loop)

	BeatsAndBars() (int64, int64)
	Plan(bars int64, seq Sequenceable)

	SettingNotifier(handler func(control LoopController))
}

type Replaceable interface {
	// Returns a new value in which any occurrences of "from" are replaced by "to".
	Replaced(from, to Sequenceable) Sequenceable
}

type Valueable interface {
	Value() interface{}
}

type Inspectable interface {
	Inspect(i Inspection)
}

type Playable interface {
	Play(ctx Context) error
}

type VariableStorage interface {
	NameFor(value interface{}) string
	Get(key string) (interface{}, bool)
	Put(key string, value interface{})
	Delete(key string)
	Variables() map[string]interface{}
}

type Context interface {
	Control() LoopController
	Device() AudioDevice
	Variables() VariableStorage
	Environment() map[string]string
}

// WorkingDirectory is a key in a context environment.
const WorkingDirectory = "pwd"

type Evaluatable interface {
	Evaluate() error
}
