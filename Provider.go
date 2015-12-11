package sip

import "sync"

////////////////////Interface//////////////////////////////

type Provider interface {
	AddTransport(t Transport)
	GetTransports() []Transport
	RemoveTransport(t Transport)

	//AddListener(l Listener)
	//RemoveListener(l Listener)
}

////////////////////Implementation////////////////////////

type provider struct {
	//listeners       map[Listener]Listener
	transports map[Transport]Transport
	//transactions   		map[Transaction]Transaction

	//forward   chan Message
	//join  	chan Transaction
	//leave 	chan Transaction

	quit      chan bool
	waitGroup *sync.WaitGroup

	tracer Tracer
}

func newProvider(tracer Tracer) *provider {
	this := &provider{}

	//this.listeners = make(map[Listener]Listener)
	this.transports = make(map[Transport]Transport)
	//this.transactions = make(map[Transaction]Transaction)

	//this.forward 	= make(chan Message)
	//this.join 	= make(chan Transaction)
	//this.leave 	= make(chan Transaction)

	this.quit = make(chan bool)
	this.waitGroup = &sync.WaitGroup{}

	this.tracer = tracer

	return this
}

func (this *provider) AddTransport(t Transport) {
	this.transports[t] = t
}

func (this *provider) GetTransports() []Transport {
	ts := make([]Transport, len(this.transports))

	l := 0
	for _, t := range this.transports {
		ts[l] = t
		l++
	}

	return ts
}

func (this *provider) RemoveTransport(t Transport) {
	delete(this.transports, t)
}

func (this *provider) Run() {
	//	for _, t := range this.transports {
	//		if err := t.Listen(); err != nil {
	//			this.tracer.Printf("Listening %s://%s:%d Failed!!!\n", t.GetNetwork(), t.GetAddress(), t.GetPort())
	//		} else {
	//			this.tracer.Printf("Listening %s://%s:%d Runing...\n", t.GetNetwork(), t.GetAddress(), t.GetPort())
	//			this.waitGroup.Add(1)
	//			go this.ServeAccept(t.(*transport))
	//		}
	//	}

	//	//infinite loop run until ctrl+c
	//	for {
	//		select {
	//		case s := <-this.join:
	//			this.transactions[s] = s
	//		case s := <-this.leave:
	//			delete(this.transactions, s)
	//		case msg := <-this.forward:
	//			for _, s := range this.transactions {
	//				if err := s.Forward(msg); err != nil {
	//					this.tracer.Println(err)
	//					for _, l := range this.listeners {
	//						l.ProcessIOException(newEventIOException(s, s.conn.RemoteAddr()))
	//					}
	//				}
	//			}
	//		case <-this.quit:
	//			this.tracer.Println("Provider Quit")
	//			return
	//		}
	//	}
}

func (this *provider) Stop() {
	//	for _, s := range this.transactions {
	//		s.Close()
	//	}
	//	for _, t := range this.transports {
	//		t.Close()
	//	}
	//	close(this.quit)
	//	this.waitGroup.Wait()
}