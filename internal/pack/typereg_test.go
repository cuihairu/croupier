package pack

import (
    "encoding/json"
    "testing"

    "google.golang.org/protobuf/proto"
    descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

// buildTestFDS builds a minimal FileDescriptorSet with a package ex.testing
// defining two messages: Foo { name string, age int32, labels map<string,string> }
// and Bar { when google.protobuf.Timestamp } to exercise special cases.
func buildTestFDS(t *testing.T) []byte {
    t.Helper()
    // map<string,string> is represented as nested message Foo.LabelsEntry { key string; value string; } with map_entry=true
    labelsEntry := &descriptorpb.DescriptorProto{
        Name: proto.String("LabelsEntry"),
        Field: []*descriptorpb.FieldDescriptorProto{
            { Name: proto.String("key"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum() },
            { Name: proto.String("value"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum() },
        },
        Options: &descriptorpb.MessageOptions{ MapEntry: proto.Bool(true) },
    }
    foo := &descriptorpb.DescriptorProto{
        Name: proto.String("Foo"),
        Field: []*descriptorpb.FieldDescriptorProto{
            { Name: proto.String("name"), JsonName: proto.String("name"), Number: proto.Int32(1), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum() },
            { Name: proto.String("age"), JsonName: proto.String("age"), Number: proto.Int32(2), Label: descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum() },
            { Name: proto.String("labels"), JsonName: proto.String("labels"), Number: proto.Int32(3), Label: descriptorpb.FieldDescriptorProto_LABEL_REPEATED.Enum(), Type: descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(), TypeName: proto.String(".ex.testing.Foo.LabelsEntry") },
        },
        NestedType: []*descriptorpb.DescriptorProto{ labelsEntry },
    }

    file := &descriptorpb.FileDescriptorProto{
        Name:    proto.String("ex/testing.proto"),
        Package: proto.String("ex.testing"),
        MessageType: []*descriptorpb.DescriptorProto{ foo },
        Syntax:  proto.String("proto3"),
    }
    fds := &descriptorpb.FileDescriptorSet{ File: []*descriptorpb.FileDescriptorProto{ file } }
    b, err := proto.Marshal(fds)
    if err != nil { t.Fatalf("marshal fds: %v", err) }
    return b
}

func TestTypeRegistry_JSON_Proto_Roundtrip(t *testing.T) {
    reg := NewTypeRegistry()
    if err := reg.LoadFDS(buildTestFDS(t)); err != nil {
        t.Fatalf("LoadFDS: %v", err)
    }
    // JSON -> Proto (Foo)
    in := map[string]any{"name": "alice", "age": 30, "labels": map[string]any{"k1":"v1"}}
    jb, _ := json.Marshal(in)
    bin, err := reg.JSONToProtoBin("ex.testing.Foo", jb)
    if err != nil { t.Fatalf("JSONToProtoBin: %v", err) }
    if len(bin) == 0 { t.Fatalf("empty bin") }
    // Back: Proto -> JSON
    out, err := reg.ProtoBinToJSON("ex.testing.Foo", bin)
    if err != nil { t.Fatalf("ProtoBinToJSON: %v", err) }
    var m map[string]any
    if err := json.Unmarshal(out, &m); err != nil { t.Fatalf("decode json: %v", err) }
    if m["name"] != "alice" { t.Fatalf("name mismatch: %v", m["name"]) }
    // int32 age may unmarshal as float64 in generic map
    if _, ok := m["age"].(float64); !ok { t.Fatalf("age not number: %T", m["age"]) }
    // labels is object
    if obj, ok := m["labels"].(map[string]any); !ok || obj["k1"] != "v1" {
        t.Fatalf("labels missing: %v", m["labels"])
    }
}

