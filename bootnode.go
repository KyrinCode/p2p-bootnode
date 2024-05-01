package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"

	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"

	// b58 "github.com/mr-tron/base58/base58" // test
	dht "github.com/libp2p/go-libp2p-kad-dht"
	drouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	dutil "github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
)

const Rendezvous = "room-pubsub-kaddht"

func main() {
	ctx := context.Background()

	// create a new libp2p Host that listens on a random TCP port
	// we can specify port like /ip4/0.0.0.0/tcp/3326
	host, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/0.0.0.0/tcp/54321"))
	if err != nil {
		panic(err)
	}

	// view host details and addresses
	// fmt.Printf("[Host ID] %s\n", host.ID().Pretty())
	// fmt.Printf("Assigned listening addresses:\n")
	// for _, addr := range host.Addrs() {
	// 	fmt.Printf("%s\n", addr.String())
	// }
	// fmt.Printf("\n")

	fmt.Printf("%s/p2p/%s\n", host.Addrs()[0], host.ID().Pretty())

	// setup DHT with empty discovery peers
	// so this will be a discovery peer for others
	// this peer should run on cloud(with public ip address)
	discoveryPeers := []multiaddr.Multiaddr{}

	dht, err := initDHT(ctx, host, discoveryPeers)
	if err != nil {
		panic(err)
	}

	// setup peer discovery
	go Discover(ctx, host, dht, Rendezvous)

	for {
		time.Sleep(2 * time.Minute)
	}
}

func initDHT(ctx context.Context, host host.Host, bootstrapPeers []multiaddr.Multiaddr) (*dht.IpfsDHT, error) {
	var options []dht.Option

	// if no bootstrap peers give this peer act as a bootstraping node
	// other peers can use this peers ipfs address for peer discovery via dht
	if len(bootstrapPeers) == 0 {
		options = append(options, dht.Mode(dht.ModeServer))
	}

	kademliaDHT, err := dht.New(ctx, host, options...)
	if err != nil {
		return nil, err
	}

	if err = kademliaDHT.Bootstrap(ctx); err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	for _, peerAddr := range bootstrapPeers {
		peerinfo, _ := peer.AddrInfoFromP2pAddr(peerAddr)

		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := host.Connect(ctx, *peerinfo); err != nil {
				log.Printf("Error while connecting to node %q: %-v", peerinfo, err)
			} else {
				log.Printf("Connection established with bootstrap node: %q", *peerinfo)
			}
		}()
	}
	wg.Wait()

	return kademliaDHT, nil
}

func Discover(ctx context.Context, h host.Host, dht *dht.IpfsDHT, rendezvous string) {
	var routingDiscovery = drouting.NewRoutingDiscovery(dht)

	dutil.Advertise(ctx, routingDiscovery, rendezvous)

	ticker := time.NewTicker(time.Second * 1)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:

			peers, err := routingDiscovery.FindPeers(ctx, rendezvous)
			if err != nil {
				log.Fatal(err)
			}

			for p := range peers {
				if p.ID == h.ID() {
					continue
				}
				// err := h.Connect(ctx, p)
				// if err != nil {
				// 	fmt.Printf("Failed connecting to %s, error: %s\n", p.ID, err)
				// } else {
				// 	fmt.Printf("Connected to peer: %s\n", p.ID)
				// }
				if h.Network().Connectedness(p.ID) != network.Connected {
					_, err = h.Network().DialPeer(ctx, p.ID)
					if err != nil {
						fmt.Printf("Failed connecting to %s, error: %s\n", p.ID.String(), err)
						// err = h.Network().ClosePeer(p.ID)
						// if err != nil {
						// 	fmt.Printf("Failed disconnecting to %s, error: %s\n", p.ID.String(), err)
						// }
						continue
					} else {
						fmt.Printf("Connected to peer: %s with dht\n", p.ID.String())
					}
				}
			}
		}
	}
}
