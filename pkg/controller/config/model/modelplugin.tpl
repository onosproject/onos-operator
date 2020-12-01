package main

import (
	"fmt"

	_ "github.com/golang/protobuf/proto"
	"github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/goyang/pkg/yang"
	_ "github.com/openconfig/ygot/genutil"
	_ "github.com/openconfig/ygot/ygen"
	"github.com/openconfig/ygot/ygot"
	_ "github.com/openconfig/ygot/ytypes"
)

const (
    modelType = {{ .Model.Type | quote }}
    modelVersion = {{ .Model.Version | quote }}
    moduleName = {{ .Module.Name | quote }}
)

var ModelPlugin modelPlugin

type modelPlugin string

var modelData = []*gnmi.ModelData{
    {{- range .Model.Data }}
	{Name: {{ .Name | quote }}, Organization: {{ .Organization | quote }}, Version: {{ .Version | quote }}},
	{Name: {{ .Name | quote }}, Organization: {{ .Organization | quote }}, Version: {{ .Version | quote }}},
	{{- end }}
}

func (m modelPlugin) ModelData() (string, string, []*gnmi.ModelData, string) {
	return modelType, modelVersion, modelData, moduleName
}

func (m modelPlugin) UnmarshalConfigValues(jsonTree []byte) (*ygot.ValidatedGoStruct, error) {
	device := &Device{}
	vgs := ygot.ValidatedGoStruct(device)

	if err := Unmarshal([]byte(jsonTree), device); err != nil {
		return nil, err
	}

	return &vgs, nil
}

func (m modelPlugin) Validate(ygotModel *ygot.ValidatedGoStruct, opts ...ygot.ValidationOption) error {
	deviceDeref := *ygotModel
	device, ok := deviceDeref.(*Device)
	if !ok {
		return fmt.Errorf("unable to convert model")
	}
	return device.Validate()
}

func (m modelPlugin) Schema() (map[string]*yang.Entry, error) {
	return UnzipSchema()
}

// GetStateMode returns an int - we do not use the enum because we do not want a
// direct dependency on onos-config code (for build optimization)
func (m modelPlugin) GetStateMode() int {
	return 0 // modelregistry.GetStateNone
}