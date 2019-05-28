package rtns

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"

	cfg "github.com/RTradeLtd/config/v2"
	pb "github.com/RTradeLtd/grpc/krab"
	kaas "github.com/RTradeLtd/kaas/v2"
	lp "github.com/RTradeLtd/rtns/internal/libp2p"
	"github.com/RTradeLtd/rtns/mocks"
	crypto "github.com/libp2p/go-libp2p-crypto"
	peer "github.com/libp2p/go-libp2p-peer"
	"github.com/multiformats/go-multiaddr"
)

var (
	ipfsPath1 = "/ipfs/QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv"
	ipfsPath2 = "QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv"
)

// TODO:
// we need to configure fakes for the krab client
// so that we may spoof a valid krab backend

func Test_New_Publisher(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	//////////////////
	// setup mocks //
	////////////////

	pk1 := newPK(t)
	pk1Bytes, err := pk1.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	pk2 := newPK(t)
	pk2Bytes, err := pk2.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	fkb := &mocks.FakeServiceClient{}
	fkb.GetPrivateKeyReturnsOnCall(0, &pb.Response{Status: "Ok", PrivateKey: pk1Bytes}, nil)
	fkb.GetPrivateKeyReturnsOnCall(1, &pb.Response{Status: "Ok", PrivateKey: pk2Bytes}, nil)

	fns := &mocks.FakeNameSystem{}
	fns.PublishReturnsOnCall(0, nil)
	fns.PublishReturnsOnCall(1, nil)
	fns.PublishReturnsOnCall(2, errors.New("publish failed"))

	//////////////////////
	// setup publisher //
	////////////////////

	rtns := newRTNS(ctx, t, fkb, fns)
	defer rtns.Close()
	rtns.Bootstrap(lp.DefaultBootstrapPeers())

	//////////////////
	// start tests //
	////////////////

	// ensure no previous records have been published
	if err := rtns.republishEntries(); err != errNoRecordsPublisher {
		t.Fatal("wrong error received")
	}

	if err := rtns.Publish(ctx, pk1, "pk1", ipfsPath1); err != nil {
		t.Fatal(err)
	}
	if len(rtns.cache.List()) != 1 {
		fmt.Println("cache length:", len(rtns.cache.List()))
		t.Fatal("invalid cache length")
	}
	pid, err := peer.IDFromPublicKey(pk1.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("pk1", pid.String())

	if err := rtns.Publish(ctx, pk2, "pk2", ipfsPath2); err != nil {
		t.Fatal(err)
	}
	if len(rtns.cache.List()) != 2 {
		fmt.Println("cache length:", len(rtns.cache.List()))
		t.Fatal("invalid cache length")
	}
	pid, err = peer.IDFromPublicKey(pk2.GetPublic())
	if err != nil {
		t.Fatal(err)
	}
	fmt.Println("pk2", pid.String())

	if err := rtns.republishEntries(); err != nil {
		t.Fatal(err)
	}

	if err := rtns.Publish(ctx, pk2, "pk2", ipfsPath2); err == nil {
		t.Fatal("error expected")
	}
}

func Test_Keystore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	fkb := &mocks.FakeServiceClient{}
	pk := newPK(t)
	pkBytes, err := pk.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	fkb.HasPrivateKeyReturnsOnCall(0, &pb.Response{Status: "OK"}, nil)
	fkb.HasPrivateKeyReturnsOnCall(1, &pb.Response{Status: "BAD"}, errors.New("no key"))
	fkb.GetPrivateKeyReturnsOnCall(0, &pb.Response{Status: "OK", PrivateKey: pkBytes}, nil)
	fkb.GetPrivateKeyReturnsOnCall(1, &pb.Response{Status: "BAD"}, errors.New("no keys"))

	rk := NewRKeystore(ctx, &kaas.Client{ServiceClient: fkb})

	// test has
	if exists, err := rk.Has("hello"); err != nil {
		t.Fatal(err)
	} else if !exists {
		t.Fatal("key should exist")
	}
	if exists, err := rk.Has("world"); err == nil {
		t.Fatal("error expected")
	} else if exists {
		t.Fatal("key should not exist")
	}

	// test put
	if err := rk.Put("abc", nil); err == nil {
		t.Fatal("error expected")
	}

	// test get
	if pkRet, err := rk.Get("abc"); err != nil {
		t.Fatal(err)
	} else if !reflect.DeepEqual(pk, pkRet) {
		t.Fatal("keys should be equal")
	}
	if pkRet, err := rk.Get("abc"); err == nil {
		t.Fatal("error expected")
	} else if pkRet != nil {
		t.Fatal("pk should be nil")
	}

	// test delete
	if err := rk.Delete("abc"); err == nil {
		t.Fatal("error expected")
	}

	// test list
	if ids, err := rk.List(); err == nil {
		t.Fatal("error expected")
	} else if len(ids) != 0 {
		t.Fatal("bad key length returned")
	}
}

func newRTNS(ctx context.Context, t *testing.T, fkb *mocks.FakeServiceClient, fns *mocks.FakeNameSystem) *RTNS {
	pk := newPK(t)
	addr, err := multiaddr.NewMultiaddr("/ip4/0.0.0.0/tcp/4005")
	if err != nil {
		t.Fatal(err)
	}
	rtns, err := NewRTNS(ctx, newServicesConfig(), "test", pk, []multiaddr.Multiaddr{addr})
	if err != nil {
		t.Fatal(err)
	}
	rtns.keys.kb = &kaas.Client{ServiceClient: fkb}
	rtns.ns = fns
	return rtns
}
func newPK(t *testing.T) crypto.PrivKey {
	pk, _, err := crypto.GenerateKeyPair(crypto.ECDSA, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return pk
}

func newServicesConfig() cfg.Services {
	return cfg.Services{}
}
