package gol

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int  // 0 = explicit serial path (no Worker Pool)
	ImageWidth  int
	ImageHeight int
	RuleSet     int  // 0=Conway(B3/S23), 1=HighLife(B36/S23), 2=DayAndNight(B3678/S34678)
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {

	//	TODO: Put the missing channels in here.

	ioCommand := make(chan ioCommand)
	ioIdle := make(chan bool)
	ioFilename := make(chan string)
	ioOutput := make(chan uint8)
	ioInput := make(chan uint8)

	ioChannels := ioChannels{
		command:  ioCommand,
		idle:     ioIdle,
		filename: ioFilename,
		output:   ioOutput,
		input:    ioInput,
	}

	distributorChannels := DistributorChannels{
		events:     events,
		ioCommand:  ioCommand,
		ioIdle:     ioIdle,
		ioFilename: ioFilename,
		ioOutput:   ioOutput,
		ioInput:    ioInput,
	}

	// keyPresses가 nil이면 새로운 채널 생성
	if keyPresses == nil {
		keyPresses = make(chan rune)
	}

	go startIo(p, ioChannels)
	distributor(p, distributorChannels, keyPresses)
}
