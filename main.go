package main

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

type SndListRes struct {
	Format      int16
	NumMods     int16
	ModNumber   uint16
	ModInit     int32
	NumCommands int16
	Cmd         uint16
	Param1      int16
	Param2      int32
	Datapart    uint8
}

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

	namesFile, err := os.Create("names.csv")
	if err != nil {
		log.Fatal(err)
	}
	defer namesFile.Close()
	writer := csv.NewWriter(namesFile)
	defer writer.Flush()
	writer.Write([]string{"ID", "Offset", "Size", "Format", "NumMods", "ModNumber", "ModInit", "NumCommands", "Cmd", "Param1", "Param2", "Datapart"})

	numItems := uint32(len(SoundLocationMap) - 1)

	var z uint32

	for z = 1; z < numItems; z++ {
		snd := SoundLocationMap[z]
		if snd == nil {
			continue
		}

		var outPos = 0

		var raw []byte = make([]byte, snd.size)
		inbuf.Seek(int64(snd.offset), io.SeekStart)

		var tmp SndListRes
		binary.Read(inbuf, binary.BigEndian, &tmp)

		writer.Write([]string{
			fmt.Sprint(snd.id),
			fmt.Sprint(snd.offset),
			fmt.Sprint(snd.size),
			fmt.Sprint(tmp.Format),
			fmt.Sprint(tmp.NumMods),
			fmt.Sprint(tmp.ModNumber),
			fmt.Sprint(tmp.ModInit),
			fmt.Sprint(tmp.NumCommands),
			fmt.Sprint(tmp.Cmd),
			fmt.Sprint(tmp.Param1),
			fmt.Sprint(tmp.Param2),
			fmt.Sprint(tmp.Datapart),
		})

		for i := 0; i < int(snd.size); i++ {
			cTmp, err := inbuf.ReadByte()
			if err != nil {
				break
			}
			raw[outPos] = cTmp
			outPos++

		}

		fmt.Printf("%5v, ", tmp)

		fmt.Println("")
		fmt.Println("")
		fname := fmt.Sprintf("out/%v.raw", snd.id)
		os.WriteFile(fname, raw, 0677)
	}
}
