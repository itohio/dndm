package network

type SrvOptStruct struct {
}
type SrvOpt func(*SrvOptStruct) error

type DialOptStruct struct {
}
type DialOpt func(*DialOptStruct) error
