package util

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func ProtoJSONUnmarshal(data []byte, message proto.Message, discardUnknown bool) error {
	unmarshaller := &protojson.UnmarshalOptions{
		DiscardUnknown: discardUnknown,
	}

	return unmarshaller.Unmarshal(data, message)
}
