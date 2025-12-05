package livebili

import (
	"github.com/kohmebot/pkg/gopool"
	"sync"
	"time"
)

type ErrorSender struct {
	rw            sync.Mutex
	err           error
	do            func(err error)
	lastErrorTime time.Time
}

var sendDuration = 30 * time.Minute

func newErrorSender(do func(err error)) *ErrorSender {
	return &ErrorSender{do: do}
}

func (s *ErrorSender) Error(err error) {
	s.rw.Lock()
	defer func() {
		s.err = err
		s.rw.Unlock()
	}()
	if err == nil {
		return
	}
	now := time.Now()
	if s.err == nil {
		s.lastErrorTime = now
		gopool.Go(func() {
			s.do(err)
		})
		return
	}

	if now.Sub(s.lastErrorTime) > sendDuration {
		s.lastErrorTime = now
		gopool.Go(func() {
			s.do(err)
		})
	}

}
