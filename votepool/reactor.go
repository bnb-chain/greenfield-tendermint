package votepool

import (
	"fmt"

	"github.com/gogo/protobuf/proto"
	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/proto/tendermint/votepool"
)

// VotePoolChannel is used for sending and receiving votes via p2p.
const VotePoolChannel = byte(0x70)

// Reactor will 1) subscribe votes from vote pool and 2) broadcast votes to peers.
type Reactor struct {
	p2p.BaseReactor
	votePool Pool

	mtx *sync.RWMutex         // for protection of chs
	chs map[p2p.ID]chan *Vote // for subscription
}

// NewReactor returns a new Reactor with the given vote pool.
func NewReactor(votePool Pool) *Reactor {
	voteR := &Reactor{
		votePool: votePool,
		mtx:      &sync.RWMutex{},
		chs:      make(map[p2p.ID]chan *Vote, 0),
	}
	voteR.BaseReactor = *p2p.NewBaseReactor("Votepool", voteR)
	return voteR
}

// OnStart implements Service.
// A goroutine will be started to multiplex new Votes.
func (voteR *Reactor) OnStart() error {
	go voteR.subscribeVotes()
	return nil
}

// OnStop implements Service.
func (voteR *Reactor) OnStop() {
}

// SetLogger implements Service.
func (voteR *Reactor) SetLogger(l log.Logger) {
	voteR.Logger = l
}

// AddPeer implements Reactor.
// It starts a broadcast routine ensuring all local votes are forwarded to the given peer.
func (voteR *Reactor) AddPeer(peer p2p.Peer) {
	ch := make(chan *Vote)
	voteR.mtx.Lock()
	voteR.chs[peer.ID()] = ch
	voteR.mtx.Unlock()

	go voteR.broadcastVotes(peer, ch)
}

// RemovePeer implements Reactor.
func (voteR *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	voteR.mtx.Lock()
	defer voteR.mtx.Unlock()

	if ch, ok := voteR.chs[peer.ID()]; ok {
		close(ch)
		delete(voteR.chs, peer.ID())
	}
}

// GetChannels implements Reactor.
func (voteR *Reactor) GetChannels() []*conn.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  VotePoolChannel,
			Priority:            7,
			RecvMessageCapacity: 1024,
			MessageType:         &votepool.Message{},
		},
	}
}

// Receive implements Reactor.
func (voteR *Reactor) Receive(chID byte, peer p2p.Peer, msgBytes []byte) {
	msg := &votepool.Message{}
	err := proto.Unmarshal(msgBytes, msg)
	if err != nil {
		panic(err)
	}
	uw, err := msg.Unwrap()
	if err != nil {
		panic(err)
	}
	voteR.ReceiveEnvelope(p2p.Envelope{
		ChannelID: chID,
		Src:       peer,
		Message:   uw,
	})
}

func (voteR *Reactor) ReceiveEnvelope(e p2p.Envelope) {
	switch msg := e.Message.(type) {
	case *votepool.Vote:
		vote := NewVote(msg.PubKey, msg.Signature, uint8(msg.EventType), msg.EventHash)
		err := voteR.votePool.AddVote(vote)
		voteR.Logger.Info("could add vote", "vote", msg.String(), "err", err)
	default:
		voteR.Logger.Error("unknown message type", "src", e.Src, "chId", e.ChannelID, "msg", e.Message)
		voteR.Switch.StopPeerForError(e.Src, fmt.Errorf("votepool cannot handle message of type: %T", e.Message))
		return
	}
}

// broadcastVotes routine will broadcast votes to peers.
func (voteR *Reactor) broadcastVotes(peer p2p.Peer, ch chan *Vote) {
	for {
		if !voteR.IsRunning() || !peer.IsRunning() {
			return
		}

		select {
		case vote := <-ch:
			_ = p2p.SendEnvelopeShim(peer, p2p.Envelope{ //nolint: staticcheck
				ChannelID: VotePoolChannel,
				Message: &votepool.Vote{
					PubKey:    vote.PubKey,
					Signature: vote.Signature,
					EventType: uint32(vote.EvenType),
					EventHash: vote.EventHash,
				},
			}, voteR.Logger)
			voteR.Logger.Debug("sent vote", "peer", peer.ID(), "hash", vote.EventHash)
		case <-peer.Quit():
			return
		case <-voteR.Quit():
			return
		}
	}
}

// subscribeVotes routine will consume votes and multiplex to channels.
func (voteR *Reactor) subscribeVotes() {
	ch := voteR.votePool.SubscribeVotes()
	for {
		vote := <-ch
		voteR.mtx.RLock()
		//TODO: stop sending vote to the peer who gives us the vote
		for _, subCh := range voteR.chs {
			go func(ch chan *Vote) { ch <- vote }(subCh)
		}
		voteR.mtx.RUnlock()
	}
}
