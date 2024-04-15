package network

import (
	"net/url"
	"reflect"
	"testing"
)

func TestPeerFromString(t *testing.T) {
	tests := []struct {
		name    string
		want    Peer
		wantErr bool
	}{
		{
			name: "tcp://localhost:123/peer-id?value=val",
			want: Peer{
				scheme: "tcp",
				addr:   "localhost:123",
				id:     "peer-id",
				args:   url.Values{"value": []string{"val"}},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PeerFromString(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("PeerFromString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PeerFromString() = %v, want %v", got, tt.want)
			}
			if got.String() != tt.name {
				t.Errorf("PeerFromString() = %v, want %v", got, tt.name)
			}
		})
	}
}
