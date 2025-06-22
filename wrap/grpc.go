package wrap

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"text/template"

	"github.com/emicklei/proto"
	"gofr.dev/pkg/gofr"
)

const (
	filePerm                = 0644
	validationFileSuffix    = "_validation.go"
	serverFileSuffix        = "_server.go"
	serverWrapperFileSuffix = "_gofr.go"
	clientFileSuffix        = "_client.go"
	clientHealthFile        = "health_client.go"
	serverHealthFile        = "health_gofr.go"
	serverRequestFile       = "request_gofr.go"
)

var (
	ErrNoProtoFile        = errors.New("proto file path is required")
	ErrOpeningProtoFile   = errors.New("error opening the proto file")
	ErrFailedToParseProto = errors.New("failed to parse proto file")
	ErrGeneratingWrapper  = errors.New("error while generating the code using proto file")
	ErrWritingFile        = errors.New("error writing the generated code to the file")
)

var googleProtobufs = map[string]string{
	"google.protobuf.Any":                      "anypb.Any",
	"google.protobuf.Api":                      "apipb.Api",
	"google.protobuf.Method":                   "apipb.Method",
	"google.protobuf.Mixin":                    "apipb.Mixin",
	"google.protobuf.FileDescriptorSet":        "descriptorpb.FileDescriptorSet",
	"google.protobuf.FileDescriptorProto":      "descriptorpb.FileDescriptorProto",
	"google.protobuf.DescriptorProto":          "descriptorpb.DescriptorProto",
	"google.protobuf.ExtensionRangeOptions":    "descriptorpb.ExtensionRangeOptions",
	"google.protobuf.FieldDescriptorProto":     "descriptorpb.FieldDescriptorProto",
	"google.protobuf.OneofDescriptorProto":     "descriptorpb.OneofDescriptorProto",
	"google.protobuf.EnumDescriptorProto":      "descriptorpb.EnumDescriptorProto",
	"google.protobuf.EnumValueDescriptorProto": "descriptorpb.EnumValueDescriptorProto",
	"google.protobuf.ServiceDescriptorProto":   "descriptorpb.ServiceDescriptorProto",
	"google.protobuf.MethodDescriptorProto":    "descriptorpb.MethodDescriptorProto",
	"google.protobuf.FileOptions":              "descriptorpb.FileOptions",
	"google.protobuf.MessageOptions":           "descriptorpb.MessageOptions",
	"google.protobuf.FieldOptions":             "descriptorpb.FieldOptions",
	"google.protobuf.OneofOptions":             "descriptorpb.OneofOptions",
	"google.protobuf.EnumOptions":              "descriptorpb.EnumOptions",
	"google.protobuf.EnumValueOptions":         "descriptorpb.EnumValueOptions",
	"google.protobuf.ServiceOptions":           "descriptorpb.ServiceOptions",
	"google.protobuf.MethodOptions":            "descriptorpb.MethodOptions",
	"google.protobuf.UninterpretedOption":      "descriptorpb.UninterpretedOption",
	"google.protobuf.FeatureSet":               "descriptorpb.FeatureSet",
	"google.protobuf.FeatureSetDefaults":       "descriptorpb.FeatureSetDefaults",
	"google.protobuf.SourceCodeInfo":           "descriptorpb.SourceCodeInfo",
	"google.protobuf.GeneratedCodeInfo":        "descriptorpb.GeneratedCodeInfo",
	"google.protobuf.SymbolVisibility":         "descriptorpb.SymbolVisibility",
	"google.protobuf.Duration":                 "durationpb.Duration",
	"google.protobuf.Empty":                    "emptypb.Empty",
	"google.protobuf.FieldMask":                "fieldmaskpb.FieldMask",
	"google.protobuf.GoFeatures":               "gofeaturespb.GoFeatures",
	"google.protobuf.SourceContext":            "sourcecontextpb.SourceContext",
	"google.protobuf.Struct":                   "structpb.Struct",
	"google.protobuf.Value":                    "structpb.Value",
	"google.protobuf.NullValue":                "structpb.NullValue",
	"google.protobuf.ListValue":                "structpb.ListValue",
	"google.protobuf.Timestamp":                "timestamppb.Timestamp",
	"google.protobuf.Type":                     "typepb.Type",
	"google.protobuf.Field":                    "typepb.Field",
	"google.protobuf.Enum":                     "typepb.Enum",
	"google.protobuf.EnumValue":                "typepb.EnumValue",
	"google.protobuf.Option":                   "typepb.Option",
	"google.protobuf.Syntax":                   "typepb.Syntax",
	"google.protobuf.DoubleValue":              "wrapperspb.DoubleValue",
	"google.protobuf.FloatValue":               "wrapperspb.FloatValue",
	"google.protobuf.Int64Value":               "wrapperspb.Int64Value",
	"google.protobuf.UInt64Value":              "wrapperspb.UInt64Value",
	"google.protobuf.Int32Value":               "wrapperspb.Int32Value",
	"google.protobuf.UInt32Value":              "wrapperspb.UInt32Value",
	"google.protobuf.BoolValue":                "wrapperspb.BoolValue",
	"google.protobuf.StringValue":              "wrapperspb.StringValue",
	"google.protobuf.BytesValue":               "wrapperspb.BytesValue",
}

// ServiceMethod represents a method in a proto service.
type ServiceMethod struct {
	Name            string
	Request         string
	Response        string
	RawRequest      string
	RawResponse     string
	StreamsRequest  bool
	StreamsResponse bool
}

// ProtoService represents a service in a proto file.
type ProtoService struct {
	Name    string
	Methods []ServiceMethod
}

// WrapperData is the template data structure.
type WrapperData struct {
	Package  string
	Service  string
	Methods  []ServiceMethod
	Requests []ServiceRequest
	Source   string
	Imports  []string
}

type ServiceRequest struct {
	Request    string
	RawRequest string
}

type FileType struct {
	FileSuffix    string
	CodeGenerator func(*gofr.Context, *WrapperData) string
}

// BuildGRPCGoFrClient generates gRPC client wrapper code based on a proto definition.
func BuildGRPCGoFrClient(ctx *gofr.Context) (any, error) {
	gRPCClient := []FileType{
		{FileSuffix: clientFileSuffix, CodeGenerator: generateGoFrClient},
		{FileSuffix: clientHealthFile, CodeGenerator: generateGoFrClientHealth},
	}

	return generateWrapper(ctx, gRPCClient...)
}

// BuildGRPCGoFrServer generates gRPC client and server code based on a proto definition.
func BuildGRPCGoFrServer(ctx *gofr.Context) (any, error) {
	gRPCServer := []FileType{
		{FileSuffix: serverWrapperFileSuffix, CodeGenerator: generateGoFrServerWrapper},
		{FileSuffix: serverHealthFile, CodeGenerator: generateGoFrServerHealthWrapper},
		{FileSuffix: serverRequestFile, CodeGenerator: generateGoFrRequestWrapper},
		{FileSuffix: serverFileSuffix, CodeGenerator: generateGoFrServer},
		{FileSuffix: validationFileSuffix, CodeGenerator: generateGoFrServerValidator},
	}

	return generateWrapper(ctx, gRPCServer...)
}

// generateWrapper executes the function for specified FileType to create GoFr integrated
// gRPC server/client files with the required services in proto file and
// specified suffix for every service specified in the proto file.
func generateWrapper(ctx *gofr.Context, options ...FileType) (any, error) {
	protoPath := ctx.Param("proto")
	if protoPath == "" {
		ctx.Logger.Error(ErrNoProtoFile)
		return nil, ErrNoProtoFile
	}

	definition, err := parseProtoFile(ctx, protoPath)
	if err != nil {
		ctx.Logger.Errorf("Failed to parse proto file: %v", err)
		return nil, err
	}

	imports := getImports(ctx, definition, protoPath)
	projectPath, packageName := getPackageAndProject(ctx, definition, protoPath)
	services := getServices(ctx, definition)
	requests := getRequests(ctx, services)

	for _, service := range services {
		wrapperData := WrapperData{
			Package:  packageName,
			Service:  service.Name,
			Methods:  service.Methods,
			Requests: uniqueRequestTypes(ctx, service.Methods),
			Source:   path.Base(protoPath),
			Imports:  imports,
		}

		if err := generateFiles(ctx, projectPath, service.Name, &wrapperData, requests, options...); err != nil {
			return nil, err
		}
	}

	ctx.Logger.Info("Successfully generated all files for GoFr integrated gRPC servers/clients")

	return "Successfully generated all files for GoFr integrated gRPC servers/clients", nil
}

// parseProtoFile opens and parses the proto file.
func parseProtoFile(ctx *gofr.Context, protoPath string) (*proto.Proto, error) {
	file, err := os.Open(protoPath)
	if err != nil {
		ctx.Logger.Errorf("Failed to open proto file: %v", err)
		return nil, ErrOpeningProtoFile
	}
	defer file.Close()
	parser := proto.NewParser(file)

	definition, err := parser.Parse()
	if err != nil {
		ctx.Logger.Errorf("Failed to parse proto file: %v", err)
		return nil, ErrFailedToParseProto
	}

	return definition, nil
}

// generateFiles generates files for a given service.
func generateFiles(ctx *gofr.Context, projectPath, serviceName string, wrapperData *WrapperData,
	requests []ServiceRequest, options ...FileType) error {
	for _, option := range options {
		if option.FileSuffix == serverRequestFile {
			wrapperData.Requests = requests
		}

		generatedCode := option.CodeGenerator(ctx, wrapperData)
		if generatedCode == "" {
			ctx.Logger.Errorf("Failed to generate code for service %s with file suffix %s", serviceName, option.FileSuffix)
			return ErrGeneratingWrapper
		}

		outputFilePath := getOutputFilePath(projectPath, serviceName, option.FileSuffix)
		if err := os.WriteFile(outputFilePath, []byte(generatedCode), filePerm); err != nil {
			ctx.Logger.Errorf("Failed to write file %s: %v", outputFilePath, err)
			return ErrWritingFile
		}

		ctx.Logger.Infof("Generated file for service %s at %s", serviceName, outputFilePath)
	}

	return nil
}

// getOutputFilePath generates the output file path based on the file suffix.
func getOutputFilePath(projectPath, serviceName, fileSuffix string) string {
	switch fileSuffix {
	case clientHealthFile:
		return path.Join(projectPath, clientHealthFile)
	case serverHealthFile:
		return path.Join(projectPath, serverHealthFile)
	case serverRequestFile:
		return path.Join(projectPath, serverRequestFile)
	default:
		return path.Join(projectPath, strings.ToLower(serviceName)+fileSuffix)
	}
}

// getRequests extracts all unique request types from the services.
func getRequests(ctx *gofr.Context, services []ProtoService) []ServiceRequest {
	requests := make(map[string]ServiceRequest)

	for _, service := range services {
		for _, method := range service.Methods {
			requests[method.Request] = ServiceRequest{
				Request:    method.Request,
				RawRequest: method.RawRequest,
			}
		}
	}

	ctx.Logger.Debugf("Extracted unique request types: %v", requests)

	return mapValuesToSlice(requests)
}

// uniqueRequestTypes extracts unique request types from methods.
func uniqueRequestTypes(ctx *gofr.Context, methods []ServiceMethod) []ServiceRequest {
	requests := make(map[string]ServiceRequest)

	for _, method := range methods {
		requests[method.Request] = ServiceRequest{
			Request:    method.Request,
			RawRequest: method.RawRequest,
		} // Include all request types
	}

	ctx.Logger.Debugf("Extracted unique request types for methods: %v", requests)

	return mapValuesToSlice(requests)
}

// mapKeysToSlice converts a map's keys to a slice.
func mapValuesToSlice(m map[string]ServiceRequest) []ServiceRequest {
	values := make([]ServiceRequest, 0, len(m))
	for _, value := range m {
		values = append(values, value)
	}

	return values
}

// executeTemplate executes a template with the provided data.
func executeTemplate(ctx *gofr.Context, data *WrapperData, tmpl string) string {
	funcMap := template.FuncMap{
		"lowerFirst": func(s string) string {
			if s == "" {
				return ""
			}
			return strings.ToLower(s[:1]) + s[1:]
		},
	}

	tmplInstance := template.Must(template.New("template").Funcs(funcMap).Parse(tmpl))

	var buf bytes.Buffer

	if err := tmplInstance.Execute(&buf, data); err != nil {
		ctx.Logger.Errorf("Template execution failed: %v", err)
		return ""
	}

	return buf.String()
}

// Template generators.
func generateGoFrServerValidator(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, validationTemplate)
}

func generateGoFrServerWrapper(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, wrapperTemplate)
}

func generateGoFrRequestWrapper(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, messageTemplate)
}

func generateGoFrServerHealthWrapper(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, healthServerTemplate)
}

func generateGoFrClientHealth(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, clientHealthTemplate)
}

func generateGoFrServer(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, serverTemplate)
}

func generateGoFrClient(ctx *gofr.Context, data *WrapperData) string {
	return executeTemplate(ctx, data, clientTemplate)
}

// getPackageAndProject extracts the package name and project path from the proto definition.
func getPackageAndProject(ctx *gofr.Context, definition *proto.Proto, protoPath string) (projectPath, packageName string) {
	proto.Walk(definition,
		proto.WithOption(func(opt *proto.Option) {
			if opt.Name == "go_package" {
				packageName = path.Base(opt.Constant.Source)
			}
		}),
	)

	projectPath = path.Dir(protoPath)
	ctx.Logger.Debugf("Extracted package name: %s, project path: %s", packageName, projectPath)

	return projectPath, packageName
}

// getImports extracts the import directories from google protobufs and relative go_package proto definitions.
func getImports(ctx *gofr.Context, definition *proto.Proto, protoPath string) []string {
	imports := []string{}
	googleImports := map[string]string{
		"google/protobuf/any.proto":            "anypb \"google.golang.org/protobuf/types/known/anypb\"",
		"google/protobuf/api.proto":            "apipb \"google.golang.org/protobuf/types/known/apipb\"",
		"google/protobuf/descriptor.proto":     "descriptorpb \"google.golang.org/protobuf/types/descriptorpb\"",
		"google/protobuf/duration.proto":       "durationpb \"google.golang.org/protobuf/types/known/durationpb\"",
		"google/protobuf/empty.proto":          "emptypb \"google.golang.org/protobuf/types/known/emptypb\"",
		"google/protobuf/field_mask.proto":     "fieldmaskpb \"google.golang.org/protobuf/types/known/fieldmaskpb\"",
		"google/protobuf/go_features.proto":    "gofeaturespb \"google.golang.org/protobuf/types/gofeaturespb\"",
		"google/protobuf/source_context.proto": "sourcecontextpb \"google.golang.org/protobuf/types/known/sourcecontextpb\"",
		"google/protobuf/struct.proto":         "structpb \"google.golang.org/protobuf/types/known/structpb\"",
		"google/protobuf/timestamp.proto":      "timestamppb \"google.golang.org/protobuf/types/known/timestamppb\"",
		"google/protobuf/type.proto":           "typepb \"google.golang.org/protobuf/types/known/typepb\"",
		"google/protobuf/wrappers.proto":       "wrapperspb \"google.golang.org/protobuf/types/known/wrapperspb\"",
	}
	for _, elem := range definition.Elements {
		if imported, ok := elem.(*proto.Import); ok {
			if googleImport, ok := googleImports[imported.Filename]; ok {
				imports = append(imports, googleImport)
			} else {
				lastIndex := strings.LastIndex(protoPath, "/")
				newProto, err := parseProtoFile(ctx, protoPath[:lastIndex+1]+imported.Filename)
				if err != nil {
					ctx.Logger.Errorf("Failed to parse imported proto file %s: %v", imported.Filename, err)
					continue
				}
				packageSource := ""
				packageName := ""
				for _, newElem := range newProto.Elements {
					if goPackage, ok := newElem.(*proto.Option); ok && goPackage.Name == "go_package" {
						packageSource = goPackage.Constant.Source
					}
					if goPackage, ok := newElem.(*proto.Package); ok {
						packageName = goPackage.Name
					}
				}
				lastPiece := strings.LastIndex(packageName, ".")
				imports = append(imports, fmt.Sprintf("%s \"%s\"", packageName[lastPiece+1:], packageSource))
			}
		}
	}
	return imports
}

// getServices extracts services from the proto definition.
func getServices(ctx *gofr.Context, definition *proto.Proto) []ProtoService {
	var services []ProtoService

	proto.Walk(definition,
		proto.WithService(func(s *proto.Service) {
			service := ProtoService{Name: s.Name}

			for _, element := range s.Elements {
				if rpc, ok := element.(*proto.RPC); ok {
					service.Methods = append(service.Methods, ServiceMethod{
						Name:            rpc.Name,
						Request:         getProperType(rpc.RequestType),
						Response:        getProperType(rpc.ReturnsType),
						RawRequest:      getRawType(rpc.RequestType),
						RawResponse:     getRawType(rpc.ReturnsType),
						StreamsRequest:  rpc.StreamsRequest,
						StreamsResponse: rpc.StreamsReturns,
					})
				}
			}

			services = append(services, service)
		}),
	)

	ctx.Logger.Debugf("Extracted services: %v", services)

	return services
}

func getProperType(tpe string) string {
	if strings.HasPrefix(tpe, "google.protobuf.") {
		if protobuf, ok := googleProtobufs[tpe]; ok {
			return protobuf
		} else {
			return tpe
		}
	} else if strings.Contains(tpe, ".") {
		lastIndex := strings.LastIndex(tpe, ".")
		submoduleIndex := strings.LastIndex(tpe[:lastIndex], ".")
		if submoduleIndex != -1 {
			return fmt.Sprintf("%s.%s", tpe[submoduleIndex+1:lastIndex], tpe[lastIndex+1:])
		}
	}
	return tpe
}

func getRawType(tpe string) string {
	lastIndex := strings.LastIndex(tpe, ".")
	return tpe[lastIndex+1:]
}
