package main

import (
	"bytes"
	"encoding/binary"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"runtime"

	"github.com/remeh/sizedwaitgroup"
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
	rateFlag := flag.Int("sampleRate", 44100, "output sample rate in Hz")
	flag.Parse()
	sampleRate = *rateFlag

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

	swg := sizedwaitgroup.New(runtime.NumCPU())

	var z uint32

	for z = 1; z < numItems; z++ {
		snd := SoundLocationMap[z]
		if snd == nil {
			continue
		}

		raw := make([]byte, snd.size)
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

		if _, err := io.ReadFull(inbuf, raw); err != nil {
			log.Printf("failed to read sound %d: %v\n", snd.id, err)
			continue
		}

		data, srcRate, bits := parseSoundHeader(raw)
		id := snd.id
		format := tmp.Format

		swg.Add()
		go func(data []byte, id uint32, format int16, srcRate, bits int) {
			defer swg.Done()
			samples := decodeSound(data, format, srcRate, bits)
			fname := fmt.Sprintf("out/%v.wav", id)
			if err := writeWAV(fname, samples); err != nil {
				log.Printf("failed to write %s: %v\n", fname, err)
			}
		}(data, id, format, srcRate, bits)

	}

	swg.Wait()
}
