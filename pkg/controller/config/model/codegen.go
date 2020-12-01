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

const (
	moduleTplPath  = "/etc/onos/codegen/module.tpl"
	pluginTplPath  = "/etc/onos/codegen/modelplugin.tpl"
	pluginRepoPath = "/etc/onos/plugins"
)

func generatePlugin(model *v1beta1.Model) error {
	// Create the template arguments
	args := getTemplateArgs(model)

	// Get the module directory
	if pluginExists(model) {
		return nil
	}

	// Create a directory for the module
	if err := createPluginDir(model); err != nil {
		return err
	}

	// Generate the go.mod file
	log.Debugf("Generating go.mod for Model %s.%s", model.Namespace, model.Name)
	if err := applyTemplate("module.tpl", moduleTplPath, filepath.Join(getPluginDir(model), "go.mod"), args); err != nil {
		return err
	}

	// Generate the main
	log.Debugf("Generating main.go for Model %s.%s", model.Namespace, model.Name)
	if err := applyTemplate("modelplugin.tpl", pluginTplPath, filepath.Join(getPluginDir(model), "main.go"), args); err != nil {
		return err
	}

	// Write the YANG models to the temporary directory
	log.Debugf("Copying YANG models for Model %s.%s", model.Namespace, model.Name)
	if err := writeYangModels(model); err != nil {
		return err
	}

	// Generate the YANG bindings
	log.Debugf("Generating YANG bindings for Model %s.%s", model.Namespace, model.Name)
	if err := generateYangBindings(model); err != nil {
		return err
	}

	// Compile the plugin
	log.Debugf("Compiling plugin for Model %s.%s", model.Namespace, model.Name)
	if err := compilePlugin(model); err != nil {
		return err
	}
	return nil
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

func getYangDir(model *v1beta1.Model) string {
	return filepath.Join(getPluginDir(model), "yang")
}

func getYangFilePath(dir string, model v1beta1.YangModel) string {
	return filepath.Join(dir, getYangFileName(model))
}

func getYangFileName(model v1beta1.YangModel) string {
	return fmt.Sprintf("%s@%s.yang", model.Name, strings.ReplaceAll(model.Version, ".", "_"))
}

func writeYangModels(model *v1beta1.Model) error {
	dir := getYangDir(model)
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

func generateYangBindings(model *v1beta1.Model) error {
	inPath := getYangDir(model)
	outPath := getPluginDir(model)
	generatedPath := filepath.Join(outPath, "generated.go")
	args := []string{
		"run",
		"github.com/openconfig/ygot/generator",
		"-path=" + inPath,
		"-output_file=" + generatedPath,
		"-package_name=main",
		"-generate_fakeroot",
	}

	for _, yangModel := range model.Spec.YangModels {
		args = append(args, getYangFileName(yangModel))
	}

	cmd := exec.Command("go", args...)
	cmd.Dir = outPath
	cmd.Env = os.Environ()
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func getPluginName(model *v1beta1.Model) string {
	return fmt.Sprintf("%s.so.%s", model.Spec.Type, model.Spec.Version)
}

func getPluginDir(model *v1beta1.Model) string {
	return filepath.Join(pluginRepoPath, fmt.Sprintf("%s-%s", model.Namespace, model.Name))
}

func getPluginPath(model *v1beta1.Model) string {
	return filepath.Join(getPluginDir(model), getPluginName(model))
}

func createPluginDir(model *v1beta1.Model) error {
	moduleDir := getPluginDir(model)
	if _, err := os.Stat(moduleDir); os.IsNotExist(err) {
		os.MkdirAll(moduleDir, os.ModeDir)
	}
	return nil
}

func pluginExists(model *v1beta1.Model) bool {
	pluginPath := getPluginPath(model)
	if _, err := os.Stat(pluginPath); os.IsNotExist(err) {
		return false
	}
	return true
}

func compilePlugin(model *v1beta1.Model) error {
	cmd := exec.Command("go", "build", "-o", getPluginPath(model), "-buildmode=plugin", "github.com/onosproject/config-models/modelplugin/"+getModuleName(model))
	cmd.Dir = getPluginDir(model)
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func readPlugin(model *v1beta1.Model) ([]byte, error) {
	return ioutil.ReadFile(getPluginPath(model))
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
