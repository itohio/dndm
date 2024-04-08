package routers

type SubOptStruct struct {
}
type SubOpt func(*SubOptStruct) error

type PubOptStruct struct {
	blocking bool
}
type PubOpt func(*PubOptStruct) error

func Blocking(b bool) PubOpt {
	return func(o *PubOptStruct) error {
		o.blocking = b
		return nil
	}
}
