package main

const TYPE_IDREF = 0x50446635 //PDf5
const TYPE_SND = 0x736E6420   //snd

type dataLocation struct {
	offset    uint32 //Location in the file
	size      uint32 //Data size
	entryType uint32 //Data type
	id        uint32 //Data ID
}
