package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

const preAlloc = 10000

var SoundLocationMap map[uint32]*dataLocation

func main() {

	//Read Clan Lord Image file
	fmt.Println("Reading CL_Sounds file")
	data, err := os.ReadFile("CL_Sounds")
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("Reading index list")
	inbuf := bytes.NewReader(data)

	readIndex(inbuf)

	fmt.Println("Reading all TYPE_SND")
	readSounds(inbuf)

}

func readIndex(inbuf *bytes.Reader) {

	var header uint16
	var entryCount uint32
	var pad1 uint32
	var pad2 uint16

	//Read header
	binary.Read(inbuf, binary.BigEndian, &header)
	if header != 0xffff {
		log.Fatal("File header incorrect, will not proceed.")
	}

	//Get number of entries
	binary.Read(inbuf, binary.BigEndian, &entryCount)
	binary.Read(inbuf, binary.BigEndian, &pad1) // ?
	binary.Read(inbuf, binary.BigEndian, &pad2) // ?

	p := message.NewPrinter(language.English)
	p.Printf("Found %d indexes.\n", entryCount)

	SoundLocationMap = make(map[uint32]*dataLocation, preAlloc)

	var i uint32
	for i = 0; i < entryCount; i++ {
		info := dataLocation{}
		binary.Read(inbuf, binary.BigEndian, &info.offset)
		binary.Read(inbuf, binary.BigEndian, &info.size)
		binary.Read(inbuf, binary.BigEndian, &info.entryType)
		binary.Read(inbuf, binary.BigEndian, &info.id)

		if info.entryType == TYPE_SND {
			SoundLocationMap[info.id] = &info
		}
	}
}

func readSounds(inbuf *bytes.Reader) {
	os.Mkdir("out", 0755)
	numItems := uint32(len(SoundLocationMap) - 1)

	var z uint32

	for z = 1; z < numItems; z++ {
		snd := SoundLocationMap[z]
		if snd == nil {
			fmt.Println("Invalid SND")
			break
		}

		fmt.Printf("id %v, offset %v, size %v\n", snd.id, snd.offset, snd.size)

		inbuf.Seek(int64(snd.offset), io.SeekStart)

	}
}