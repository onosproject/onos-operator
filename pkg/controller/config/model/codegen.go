// Copyright 2020-present Open Networking Foundation.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package model

import (
	"fmt"
	"github.com/onosproject/onos-operator/pkg/apis/config/v1beta1"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

func generatePlugin(model *v1beta1.Model) ([]byte, error) {
	// Create the template arguments
	args := getTemplateArgs(model)

	// Create a temporary directory for the module
	moduleDir, err := createTempModuleDir(model)
	if err != nil {
		return nil, err
	}

	// Generate the go.mod file
	if err := applyTemplate("module", "/etc/onos/codegen/module.tpl", filepath.Join(moduleDir, "go.mod"), args); err != nil {
		return nil, err
	}

	// Generate the main
	if err := applyTemplate("plugin", "/etc/onos/codegen/modelplugin.tpl", filepath.Join(moduleDir, "main.go"), args); err != nil {
		return nil, err
	}

	// Write the YANG models to the temporary directory
	yangDir := filepath.Join(moduleDir, "yang")
	if err := writeYangModels(model, yangDir); err != nil {
		return nil, err
	}

	// Generate the YANG bindings
	if err := generateYangBindings(model, yangDir, moduleDir); err != nil {
		return nil, err
	}

	// Compile the plugin
	if err := compilePlugin(model, moduleDir); err != nil {
		return nil, err
	}

	// Read the compiled plugin
	bytes, err := readPlugin(model, moduleDir)
	if err != nil {
		return nil, err
	}

	// Delete the temporary directory
	os.RemoveAll(moduleDir)
	return bytes, nil
}

func getModuleName(model *v1beta1.Model) string {
	return fmt.Sprintf("%s_%s", model.Name, strings.ReplaceAll(model.Spec.Version, ".", "_"))
}

func getTemplateArgs(model *v1beta1.Model) TemplateArgs {
	data := make([]ModelData, len(model.Spec.YangModels))
	for i, yangModel := range model.Spec.YangModels {
		data[i] = ModelData{
			Name:         yangModel.Name,
			Organization: yangModel.Organization,
			Version:      yangModel.Version,
		}
	}
	moduleName := getModuleName(model)
	return TemplateArgs{
		Model: ModelArgs{
			Name:    model.Name,
			Type:    model.Spec.Type,
			Version: model.Spec.Version,
			Data:    data,
		},
		Module: ModuleArgs{
			Name: moduleName,
		},
	}
}

func createTempModuleDir(model *v1beta1.Model) (string, error) {
	moduleDir, err := ioutil.TempDir("/tmp", fmt.Sprintf("%s-%s", model.Namespace, model.Name))
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
		os.MkdirAll(moduleDir, os.ModeDir)
	}
	return moduleDir, nil
}

func applyTemplate(name, tplPath, outPath string, args TemplateArgs) error {
	var funcs template.FuncMap = map[string]interface{}{
		"quote": func(value string) string {
			return "\"" + value + "\""
		},
		"replace": func(value, search, replace string) string {
			return strings.ReplaceAll(value, search, replace)
		},
	}

	tpl, err := template.New(name).
		Funcs(funcs).
		ParseFiles(tplPath)
	if err != nil {
		return err
	}

	file, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return tpl.Execute(file, args)
}

func getYangFilePath(dir string, model v1beta1.YangModel) string {
	return filepath.Join(dir, getYangFileName(model))
}

func getYangFileName(model v1beta1.YangModel) string {
	return fmt.Sprintf("%s@%s.yang", model.Name, strings.ReplaceAll(model.Version, ".", "_"))
}

func writeYangModels(model *v1beta1.Model, dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		os.MkdirAll(dir, os.ModeDir)
	}

	for _, yangModel := range model.Spec.YangModels {
		yangPath := getYangFilePath(dir, yangModel)
		if _, err := os.Stat(yangPath); os.IsNotExist(err) {
			err := ioutil.WriteFile(yangPath, []byte(yangModel.Data), os.ModePerm)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func generateYangBindings(model *v1beta1.Model, inPath, outPath string) error {
	generatedPath := filepath.Join(outPath, "generated.go")
	args := []string{
		"run",
		"github.com/openconfig/ygot/generator",
		"-path=" + inPath,
		"-output_file=" + generatedPath,
		"-package_name=" + getModuleName(model),
		"-generate_fakeroot",
	}

	for _, yangModel := range model.Spec.YangModels {
		args = append(args, getYangFileName(yangModel))
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = outPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getPluginName(model *v1beta1.Model) string {
	return fmt.Sprintf("%s.so.%s", model.Spec.Type, model.Spec.Version)
}

func getPluginPath(model *v1beta1.Model, dir string) string {
	return filepath.Join(dir, getPluginName(model))
}

func compilePlugin(model *v1beta1.Model, dir string) error {
	cmd := exec.Command("go", "build", "-o", getPluginPath(model, dir), "-buildmode=plugin", "github.com/onosproject/config-models/modelplugin/"+getModuleName(model))
	cmd.Dir = dir
	cmd.Env = append(cmd.Env, "CGO_ENABLED=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func readPlugin(model *v1beta1.Model, dir string) ([]byte, error) {
	return ioutil.ReadFile(getPluginPath(model, dir))
}

type TemplateArgs struct {
	Model  ModelArgs
	Module ModuleArgs
}

type ModelArgs struct {
	Name    string
	Type    string
	Version string
	Data    []ModelData
}

type ModelData struct {
	Name         string
	Organization string
	Version      string
}

type ModuleArgs struct {
	Name string
}
