package pack

import (
    "fmt"
    "os"
    "path/filepath"

    "google.golang.org/protobuf/encoding/protojson"
    "google.golang.org/protobuf/proto"
    "google.golang.org/protobuf/reflect/protodesc"
    "google.golang.org/protobuf/reflect/protoregistry"
    descriptorpb "google.golang.org/protobuf/types/descriptorpb"
    "google.golang.org/protobuf/reflect/protoreflect"
    "google.golang.org/protobuf/types/dynamicpb"
)

// TypeRegistry wraps protobuf descriptor registries for dynamic (un)marshal.
type TypeRegistry struct {
    files *protoregistry.Files
    types *protoregistry.Types
}

func NewTypeRegistry() *TypeRegistry {
    return &TypeRegistry{files: new(protoregistry.Files), types: new(protoregistry.Types)}
}

// LoadFDS loads a FileDescriptorSet bytes into the registry.
func (r *TypeRegistry) LoadFDS(b []byte) error {
    var fds descriptorpb.FileDescriptorSet
    if err := proto.Unmarshal(b, &fds); err != nil { return err }
    files, err := protodesc.NewFiles(&fds)
    if err != nil { return err }
    // Merge into internal registries
    files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
        _ = r.files.RegisterFile(fd)
        // Register message types
        mdRange := func(md protoreflect.MessageDescriptor) {}
        _ = mdRange // no-op; dynamic types resolved on demand
        return true
    })
    return nil
}

// LoadFDSFromDir reads all *.pb files in dir and loads as FileDescriptorSet.
func (r *TypeRegistry) LoadFDSFromDir(dir string) error {
    return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
        if err != nil { return err }
        if info.IsDir() { return nil }
        if filepath.Ext(path) != ".pb" { return nil }
        b, err := os.ReadFile(path)
        if err != nil { return err }
        return r.LoadFDS(b)
    })
}

// JSONToProtoBin converts JSON payload into binary protobuf for the given FQN.
func (r *TypeRegistry) JSONToProtoBin(typeFQN string, jsonBytes []byte) ([]byte, error) {
    d, err := r.files.FindDescriptorByName(protoreflect.FullName(typeFQN))
    if err != nil { return nil, fmt.Errorf("type %s not found: %w", typeFQN, err) }
    md, ok := d.(protoreflect.MessageDescriptor)
    if !ok { return nil, fmt.Errorf("%s is not a message", typeFQN) }
    msg := dynamicpb.NewMessage(md)
    if err := protojson.Unmarshal(jsonBytes, msg); err != nil { return nil, err }
    return proto.Marshal(msg)
}

// ProtoBinToJSON converts binary protobuf into JSON for the given FQN.
func (r *TypeRegistry) ProtoBinToJSON(typeFQN string, bin []byte) ([]byte, error) {
    d, err := r.files.FindDescriptorByName(protoreflect.FullName(typeFQN))
    if err != nil { return nil, fmt.Errorf("type %s not found: %w", typeFQN, err) }
    md, ok := d.(protoreflect.MessageDescriptor)
    if !ok { return nil, fmt.Errorf("%s is not a message", typeFQN) }
    msg := dynamicpb.NewMessage(md)
    if err := proto.Unmarshal(bin, msg); err != nil { return nil, err }
    marshaler := protojson.MarshalOptions{EmitUnpopulated: false, UseEnumNumbers: false, UseProtoNames: false}
    return marshaler.Marshal(msg)
}

