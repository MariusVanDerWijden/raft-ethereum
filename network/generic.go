package network

import "io"

func BroadcastMsg(connections []io.Writer, msg []byte) []error {
	var errs []error
	for _, conn := range connections {
		if _, err := conn.Write(msg); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

func WriteMsg(connection io.Writer, msg []byte) error {
	_, err := connection.Write(msg)
	return err
}

func Listen(conn io.Reader, callbacks []Callback, id int) {
	for {
		buffer := make([]byte, 5*1024*1024)
		n, err := conn.Read(buffer)
		if err == nil {
			for _, callback := range callbacks {
				callback(buffer[:n], id)
			}
		}
	}
}
