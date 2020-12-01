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

var ModelPlugin = &modelPlugin{}

type Modelplugin string

const (
    modelType = {{ .Model.Type | quote }}
    modelVersion = {{ .Model.Version | quote }}
    moduleName = {{ .Module.Name | quote }}
)

var ModelData = []*gnmi.ModelData{
    {{- range .Model.Data }}
	{Name: {{ .Name | quote }}, Organization: {{ .Organization | quote }}, Version: {{ .Version | quote }}},
	{Name: {{ .Name | quote }}, Organization: {{ .Organization | quote }}, Version: {{ .Version | quote }}},
	{{- end }}
}

func (m Modelplugin) ModelData() (string, string, []*gnmi.ModelData, string) {
	return modelType, modelVersion, ModelData, moduleName
}

// UnmarshallConfigValues allows Device to implement the Unmarshaller interface
func (m Modelplugin) UnmarshalConfigValues(jsonTree []byte) (*ygot.ValidatedGoStruct, error) {
	device := &Device{}
	vgs := ygot.ValidatedGoStruct(device)

	if err := Unmarshal([]byte(jsonTree), device); err != nil {
		return nil, err
	}

	return &vgs, nil
}

func (m Modelplugin) Validate(ygotModel *ygot.ValidatedGoStruct, opts ...ygot.ValidationOption) error {
	deviceDeref := *ygotModel
	device, ok := deviceDeref.(*Device)
	if !ok {
		return fmt.Errorf("unable to convert model")
	}
	return device.Validate()
}

func (m Modelplugin) Schema() (map[string]*yang.Entry, error) {
	return UnzipSchema()
}

// GetStateMode returns an int - we do not use the enum because we do not want a
// direct dependency on onos-config code (for build optimization)
func (m Modelplugin) GetStateMode() int {
	return 0 // modelregistry.GetStateNone
}