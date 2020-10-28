package main

import (
	dbibackend "./godbi"
)

// "./dbibackend"

// "time"

type packet struct {
	Magic    [4]byte
	CMDType  uint32
	ID       uint32
	DataSize uint32
}

func main() {

	// // const buffer = await this.bufferRead(16);
	// // const magic = buffer.slice(0, 4).toString();
	// // const type = buffer.readUInt32LE(4);
	// // const id = buffer.readUInt32LE(8);
	// // const data_size = buffer.readUInt32LE(12);

	// // data := []byte{68, 66, 73, 48, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0}
	// // var dataOut dbibackend.DBICommand

	// // dataIn := packet{
	// // 	Magic:     [4]byte{68, 66, 73, 48},
	// // 	CMD_type:  0,
	// // 	Id:        1,
	// // 	Data_size: 0}

	// // buf := new(bytes.Buffer)
	// data := []byte{68, 66, 73, 48, 0, 0, 0, 0, 1, 0, 0, 0, 0, 0, 0, 0}
	// buf := bytes.NewReader(data)

	// // put single encoded data packet into writer buf
	// // automatically marshaled to type packet
	// // if err := binary.Write(buf, binary.LittleEndian, dataIn); err != nil {
	// // 	fmt.Println(err)
	// // 	return
	// // }

	// // get single encoded data packet from reader
	// // automatically unmarshal to type packet
	// var dataOut packet
	// if err := binary.Read(buf, binary.LittleEndian, &dataOut); err != nil {
	// 	fmt.Println("failed to Read:", err)
	// 	return
	// }

	// fmt.Printf("%v", dataOut)

	device, err := dbibackend.Find()

	if err != nil {
		panic(err)
	}

	defer device.Close()
}
