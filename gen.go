package dndm

//go:generate protoc -I ./proto --proto_path=./proto --go_opt=paths=source_relative --go_out=./routers/pipe/types ./proto/message.proto
