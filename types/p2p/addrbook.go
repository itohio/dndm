package p2p

import "time"

func AddrbookEntryFromPeer(peer string) *AddrbookEntry {
	return &AddrbookEntry{
		Peer:              peer,
		MaxAttempts:       10,
		DefaultBackoff:    uint64(time.Second),
		MaxBackoff:        uint64(time.Minute),
		BackoffMultiplier: 1.5,
		Attempts:          0,
		FailedAttempts:    0,
		LastSuccess:       0,
		Backoff:           0,
	}
}

func AddrbookFromPeers(peers []string) []*AddrbookEntry {
	book := make([]*AddrbookEntry, 0, len(peers))
	for _, peer := range peers {
		book = append(book, AddrbookEntryFromPeer(peer))
	}
	return book
}
