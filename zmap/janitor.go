package zmap

import "time"

type Janitor interface {
	SetJanitor(*janitor)
	Janitor() *janitor
	DeleteExpired()
}

type janitor struct {
	stop     chan bool
	Interval time.Duration
}

func (j *janitor) Run(c Janitor) {
	ticker := time.NewTicker(j.Interval)
	for {
		select {
		case <-ticker.C:
			c.DeleteExpired()
		case <-j.stop:
			ticker.Stop()
			return
		}
	}
}

func stopJanitor(c Janitor) {
	c.Janitor().stop <- true
}

func runJanitor(c Janitor, ci time.Duration) {
	j := &janitor{
		Interval: ci,
		stop:     make(chan bool),
	}
	c.SetJanitor(j)
	go j.Run(c)
}
