package charger

import "sync"

var subscribers = []chan ChargerData{}
var lock sync.RWMutex

func Subscribe() (ch chan ChargerData, close func()) {
	ch = make(chan ChargerData, 10)
	lock.Lock()
	subscribers = append(subscribers, ch)
	lock.Unlock()
	close = func() {
		lock.Lock()
		defer lock.Unlock()
		for i, sub := range subscribers {
			if sub == ch {
				subscribers = append(subscribers[:i], subscribers[i+1:]...)
				break
			}
		}
	}
	return
}

func Broadcast(data ChargerData) {
	lock.RLock()
	subs := subscribers[:]
	lock.RUnlock()
	for _, ch := range subs {
		go func(ch chan ChargerData, data ChargerData) {
			ch <- data
		}(ch, data)
	}
}
