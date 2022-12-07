package votepool

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogo/protobuf/proto"
	lru "github.com/hashicorp/golang-lru"

	"github.com/tendermint/tendermint/libs/log"
	"github.com/tendermint/tendermint/libs/sync"
	"github.com/tendermint/tendermint/p2p"
	"github.com/tendermint/tendermint/p2p/conn"
	"github.com/tendermint/tendermint/proto/tendermint/votepool"
	"github.com/tendermint/tendermint/types"
)

const (
	// VotePoolChannel is the p2p channel used for sending and receiving votes in vote Pool.
	VotePoolChannel = byte(0x70)

	// Max number of peers to connected to, when there are more peers, it will stop broadcasting votes to the new ones.
	maxNumberOfPeers = 1024

	// Max number of kept vote histories from each peer, to avoiding broadcasting duplicated votes to a peer.
	maxVoteHistoryOfEachPeer = 512
)

var eventVotePoolAdded = types.QueryForEvent(eventBusVotePoolUpdates)

// Reactor will 1) subscribe votes from vote Pool and 2) broadcast votes to peers.
type Reactor struct {
	p2p.BaseReactor
	votePool VotePool

	mtx     *sync.RWMutex         // for protection of chs
	chs     map[p2p.ID]chan *Vote // for subscription
	history map[p2p.ID]*lru.Cache // to keep votes from peers

	eventBus *types.EventBus
}

// NewReactor returns a new Reactor with the given vote Pool.
func NewReactor(votePool VotePool, eventBus *types.EventBus) *Reactor {
	voteR := &Reactor{
		votePool: votePool,
		mtx:      &sync.RWMutex{},
		chs:      make(map[p2p.ID]chan *Vote, 0),
		history:  make(map[p2p.ID]*lru.Cache, 0),
		eventBus: eventBus,
	}
	voteR.BaseReactor = *p2p.NewBaseReactor("VotePool", voteR)
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
	peerID := peer.ID()
	voteR.mtx.Lock()
	defer voteR.mtx.Unlock()

	if len(voteR.chs) < maxNumberOfPeers {
		ch := make(chan *Vote)
		voteR.chs[peerID] = ch
		go voteR.broadcastVotes(peer, ch)

		// there is no error when the size parameter is positive
		c, _ := lru.New(maxVoteHistoryOfEachPeer)
		voteR.history[peerID] = c
	}
}

// RemovePeer implements Reactor.
func (voteR *Reactor) RemovePeer(peer p2p.Peer, reason interface{}) {
	peerID := peer.ID()
	voteR.mtx.Lock()
	defer voteR.mtx.Unlock()

	if _, ok := voteR.chs[peerID]; ok {
		delete(voteR.chs, peerID)
		voteR.history[peerID].Purge()
		delete(voteR.history, peerID)
	}
}

// GetChannels implements Reactor.
func (voteR *Reactor) GetChannels() []*conn.ChannelDescriptor {
	return []*p2p.ChannelDescriptor{
		{
			ID:                  VotePoolChannel,
			Priority:            7,
			RecvMessageCapacity: 256, // size is bigger than Vote message
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
		if err := voteR.votePool.AddVote(vote); err != nil {
			voteR.Logger.Info("Could not add vote", "vote", msg.String(), "err", err)
		} else {
			// keep track of votes from the remote peer
			if _, ok := voteR.history[e.Src.ID()]; ok {
				voteR.history[e.Src.ID()].Add(vote.Key(), struct{}{})
			}
		}
	default:
		voteR.Logger.Error("Unknown message type", "src", e.Src, "chId", e.ChannelID, "msg", e.Message)
		voteR.Switch.StopPeerForError(e.Src, fmt.Errorf("votepool cannot handle message of type: %T", e.Message))
		voteR.RemovePeer(e.Src, errors.New("invalid message type for vote pool channel"))
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
					EventType: uint32(vote.EventType),
					EventHash: vote.EventHash,
				},
			}, voteR.Logger)
			voteR.Logger.Debug("Sent vote to", "peer", peer.ID(), "hash", vote.EventHash)
		case <-peer.Quit():
			return
		case <-voteR.Quit():
			return
		}
	}
}

// subscribeVotes routine will consume votes from VotePool and multiplex to peer channels.
func (voteR *Reactor) subscribeVotes() {
	sub, err := voteR.eventBus.Subscribe(context.Background(), "VotePoolReactor", eventVotePoolAdded, eventBusSubscribeCap)
	if err != nil {
		panic(err)
	}
	for {
		select {
		case voteData := <-sub.Out():
			vote := voteData.Data().(Vote)
			voteR.mtx.RLock()
			for peer, subCh := range voteR.chs {
				// if the vote is received from a remote peer, no need to re-send it to the remote peer
				if !voteR.history[peer].Contains(vote.Key()) {
					go func(ch chan *Vote, vote *Vote) { ch <- vote }(subCh, &vote)
				}
			}
			voteR.mtx.RUnlock()
		case <-sub.Cancelled():
			return
		}
	}
}
