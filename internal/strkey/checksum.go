package strkey

// checksum16 is CRC16-XModem: polynomial x16+x12+x5+1, initial value 0.
// SEP-23 appends the result little-endian after the payload.
func checksum16(data []byte) uint16 {
	var crc uint16
	for _, c := range data {
		crc ^= uint16(c) << 8
		for i := 0; i < 8; i++ {
			if crc&0x8000 != 0 {
				crc = crc<<1 ^ 0x1021
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}
