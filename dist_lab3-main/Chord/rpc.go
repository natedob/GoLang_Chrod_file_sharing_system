package Chord

import "math/big"

type SendArgs struct {
	GetSuccessorRequest     bool
	GetPredecessorRequest   bool
	Notify                  bool
	CheckSucORPredFail      bool
	StoreFileRequest        bool
	GetSuccessorListRequest bool
	GetAllRequest           bool
	SendArg                 big.Int
	SendArgString           string
	Mrequest                bool
	PutAllRequest           bool
	SendBucket              map[string][]File
	File                    File
	GetIdentifier           bool
}
type ReceiveArgs struct {
	Answer              bool
	ReplyArgs           string
	FindSuccessorAnswer FindSuccessorAnswer
	ReplyInt            int
	SuccessorList       []string
	SendBucket          map[string][]File
}

// Structs for different answers
type FindSuccessorAnswer struct {
	IsSuccessor bool
	Address     string
}

type File struct {
	ID       big.Int
	FileName string
	Content  []byte
}
