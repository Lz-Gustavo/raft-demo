package recovlog

// Operation ...
type Operation int

// Different Operation types indexes, must modify it to accept different index based
// on an application semantics (readed from config file)
const (
	Read Operation = iota
	Write
	Remove
	Swap
)

// A Command represents the records saved on log file
type Command struct {
	id      int
	ts      string
	cmd     Operation
	comment string
}

// Logger is the interface that each log implements
type Logger interface {

	// Classical insert, just append a new record on the file
	Put(id int, op Operation, comment string, timeStamp string)

	// Returns a record with the corresponding id (not removing it)
	Get(id int)

	// Syncronizhes the state of all log files in different replicas if is h.Rep() is true
	Sync()

	// Like Shreading a sheet of paper, log.Shred(1) divides the log file into two parts
	Shred(times int)

	// Log destructor
	Close()

	Test()
}

// Log ...
type Log struct {
	h *handler
}

// New function instantiates a new logger daemon with default handler and configured by
// "config.toml" file
func New() *Log {

	hand := new(handler)
	hand.New("config.toml")

	child := &Log{hand}
	return child
}

// Close ...
func (l *Log) Close() {
	l.h.Delete()
}

// Put ...
func (l *Log) Put(id int, op Operation, comment string, timeStamp string) {
	l.h.Write(Command{
		id:      id,
		cmd:     op,
		comment: comment,
		ts:      timeStamp,
	})
}

// Get ...
func (l *Log) Get(id int) {

}

// Sync ...
func (l *Log) Sync() {

}

// Shred ...
func (l *Log) Shred(times int) {

}

// Test ...
func (l *Log) Test() {
	l.h.Debug()
}
