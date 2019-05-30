package rtns

import (
	"context"
	"fmt"
	"time"

	kaas "github.com/RTradeLtd/kaas/v2"
	lp "github.com/RTradeLtd/rtns/internal/libp2p"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	"github.com/ipfs/go-ipfs/namesys"
	"github.com/ipfs/go-path"
	ci "github.com/libp2p/go-libp2p-crypto"
	host "github.com/libp2p/go-libp2p-host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	peerstore "github.com/libp2p/go-libp2p-peerstore"
	"github.com/libp2p/go-libp2p-peerstore/pstoremem"
	"github.com/multiformats/go-multiaddr"
)

// Service implements the rtns logic
// as a consumable service or library.
type Service interface {
	// service management
	Close()
	DefaultBootstrap()
	Bootstrap(peers []peerstore.PeerInfo)
	// record publishing
	Publish(ctx context.Context, pk ci.PrivKey, cache bool, keyID, content string) error
	PublishWithEOL(ctx context.Context, pk ci.PrivKey, eol time.Time, cache bool, keyID, content string) error

	// key management
	GetKey(name string) (ci.PrivKey, error)
	HasKey(name string) (bool, error)
}

// Config is used to configure the RTNS service
type Config struct {
	DSPath      string
	PK          ci.PrivKey
	ListenAddrs []multiaddr.Multiaddr
	Secret      []byte
}

// rtns manages all the needed components
// to interact with public and private IPNS
// networks.
type rtns struct {
	h     host.Host
	pk    ci.PrivKey
	d     *dht.IpfsDHT
	ds    datastore.Datastore
	ns    namesys.NameSystem
	ps    peerstore.Peerstore
	ctx   context.Context
	keys  *RKeystore
	cache *Cache
}

// NewService is used to instantiate an RTNS publisher service
func NewService(ctx context.Context, kbClient *kaas.Client, cfg Config) (Service, error) {
	return newRTNS(ctx, kbClient, cfg)
}

// NewRTNS is used to instantiate our RTNS service, and start the republisher
// intended to be used as a `Publisher` type by external libraries
func newRTNS(ctx context.Context, kbClient *kaas.Client, cfg Config) (*rtns, error) {
	ds := dssync.MutexWrap(datastore.NewMapDatastore())
	ps := pstoremem.NewPeerstore()
	ht, dt, err := lp.SetupLibp2p(ctx, cfg.PK, cfg.Secret, cfg.ListenAddrs, ps, ds)
	if err != nil {
		return nil, err
	}
	r := &rtns{
		h:     ht,
		d:     dt,
		pk:    cfg.PK,
		ds:    ds,
		ps:    ps,
		ns:    namesys.NewNameSystem(dt, ds, 128),
		ctx:   ctx,
		keys:  NewRKeystore(ctx, kbClient),
		cache: NewCache(),
	}
	go r.startRepublisher()
	return r, nil
}

// Close is used to close all service needed by our publisher
func (r *rtns) Close() {
	if err := r.d.Close(); err != nil {
		fmt.Println("error shutting down dht:", err.Error())
	}
	if err := r.h.Close(); err != nil {
		fmt.Println("error shutting down host:", err.Error())
	}
	if err := r.ds.Close(); err != nil {
		fmt.Println("error shutting down datastore:", err.Error())
	}
}

// Publish is used to publish content with a fixed lifetime and ttl
func (r *rtns) Publish(ctx context.Context, pk ci.PrivKey, cache bool, keyID, content string) error {
	if cache {
		r.cache.Set(keyID)
	}
	return r.ns.Publish(ctx, pk, path.FromString(content))
}

// PublishWithEOL is used to publish an IPNS record with non default lifetime values
func (r *rtns) PublishWithEOL(ctx context.Context, pk ci.PrivKey, eol time.Time, cache bool, keyID, content string) error {
	if cache {
		r.cache.Set(keyID)
	}
	return r.ns.PublishWithEOL(ctx, pk, path.FromString(content), eol)
}

// GetKey is used to retrieve a key from the
// underlying keystore
func (r *rtns) GetKey(name string) (ci.PrivKey, error) {
	return r.keys.Get(name)
}

// HasKey is used to check if the underlying keystore
// contains the desired key
func (r *rtns) HasKey(name string) (bool, error) {
	return r.keys.Has(name)
}
