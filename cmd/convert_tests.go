package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	xfnproto "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/spf13/cobra"
	"github.com/vshn/appcat/v4/apis/fn/io/v1alpha1"
	"google.golang.org/protobuf/types/known/structpb"
	"sigs.k8s.io/yaml"
)

// ConvertCMD specifies the cobra command for triggering the maintenance.
var (
	ConvertCMD = newConvertCMD()
	file       string
)

func newConvertCMD() *cobra.Command {

	command := &cobra.Command{
		Use:   "convert",
		Short: "Convert legacy FunctionIO yaml to RunFunctionRequest yaml files",
		Long: `
The FunctionIO format does not exist anymore as of Crossplane 1.14. This small tool will help convert them to the new
RunFunctionRequest format.`,
		RunE: executeConversion,
	}

	command.Flags().StringVar(&file, "file", "", "Legacy functionIO to convert to a RunFunctionRequest")

	return command
}

func executeConversion(_ *cobra.Command, _ []string) error {

	funcIO := &v1alpha1.FunctionIO{}
	req := &xfnproto.RunFunctionRequest{}

	content, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(content, funcIO)
	if err != nil {
		return err
	}

	desired := &xfnproto.State{
		Resources: map[string]*xfnproto.Resource{},
	}

	observed := &xfnproto.State{
		Resources: map[string]*xfnproto.Resource{},
	}

	for _, obj := range funcIO.Desired.Resources {
		jsonObj, err := json.Marshal(obj.Resource)
		if err != nil {
			return err
		}

		v := map[string]interface{}{}

		err = json.Unmarshal(jsonObj, &v)
		if err != nil {
			return err
		}

		resourceStruct, err := structpb.NewStruct(v)
		if err != nil {
			return err
		}

		desired.Resources[obj.Name] = &xfnproto.Resource{Resource: resourceStruct}

	}

	for _, obj := range funcIO.Observed.Resources {
		jsonObj, err := json.Marshal(obj.Resource)
		if err != nil {
			return err
		}

		v := map[string]interface{}{}

		err = json.Unmarshal(jsonObj, &v)
		if err != nil {
			return err
		}

		resourceStruct, err := structpb.NewStruct(v)
		if err != nil {
			return err
		}

		observed.Resources[obj.Name] = &xfnproto.Resource{Resource: resourceStruct}

	}

	desCompBytes := funcIO.Desired.Composite.Resource.Raw
	if len(desCompBytes) > 0 {
		v := map[string]interface{}{}

		err = json.Unmarshal(desCompBytes, &v)
		if err != nil {
			return err
		}

		desComp, err := structpb.NewStruct(v)
		if err != nil {
			return err
		}

		// comp functions aren't allowed anymore to change the spec of the
		// composite. So the desired composite only contains changes to the
		// status or metadata.
		observed.Composite = &xfnproto.Resource{Resource: desComp}
		desired.Composite = &xfnproto.Resource{Resource: desComp}
	}

	if funcIO.Config != nil {
		configBytes := funcIO.Config.Raw
		if len(configBytes) > 0 {
			v := map[string]interface{}{}
			err := yaml.Unmarshal(configBytes, &v)
			if err != nil {
				return err
			}

			input, err := structpb.NewStruct(v)
			if err != nil {
				return err
			}

			req.Input = input
		}
	}

	req.Desired = desired
	req.Observed = observed

	yamlReq, err := yaml.Marshal(req)
	if err != nil {
		return err
	}

	fmt.Println(string(yamlReq))

	return nil
}
