package xmlstruct

import (
	"encoding/xml"
	"errors"
	"fmt"
	"go/format"
	"io"
	"os"
	"sort"
	"strings"

	"golang.org/x/exp/maps"
	"golang.org/x/exp/slices"
	"golang.org/x/net/html/charset"
)

// A Generator observes XML documents and generates Go structs into which the
// XML documents can be unmarshalled.
type Generator struct {
	charDataFieldName            string
	exportNameFunc               ExportNameFunc
	formatSource                 bool
	header                       string
	intType                      string
	nameFunc                     NameFunc
	namedTypes                   bool
	order                        int
	packageName                  string
	preserveOrder                bool
	timeLayout                   string
	topLevelAttributes           bool
	typeOrder                    map[xml.Name]int
	usePointersForOptionalFields bool
	typeElements                 map[xml.Name]*element
}

// A GeneratorOption sets an option on a Generator.
type GeneratorOption func(*Generator)

// WithCharDataFieldName sets the char data field name.
func WithCharDataFieldName(charDataFieldName string) GeneratorOption {
	return func(g *Generator) {
		g.charDataFieldName = charDataFieldName
	}
}

// WithExportNameFunc sets the export name function for the generated Go source.
func WithExportNameFunc(exportNameFunc ExportNameFunc) GeneratorOption {
	return func(g *Generator) {
		g.exportNameFunc = exportNameFunc
	}
}

// WithFormatSource sets whether to format the generated Go source.
func WithFormatSource(formatSource bool) GeneratorOption {
	return func(g *Generator) {
		g.formatSource = formatSource
	}
}

// WithHeader sets the header of the generated Go source.
func WithHeader(header string) GeneratorOption {
	return func(g *Generator) {
		g.header = header
	}
}

// WithIntType sets the int type in the generated Go source.
func WithIntType(intType string) GeneratorOption {
	return func(g *Generator) {
		g.intType = intType
	}
}

// WithNameFunc sets the name function.
func WithNameFunc(nameFunc NameFunc) GeneratorOption {
	return func(g *Generator) {
		g.nameFunc = nameFunc
	}
}

// WithNamedTypes sets whether all to generate named types for all elements.
func WithNamedTypes(namedTypes bool) GeneratorOption {
	return func(o *Generator) {
		o.namedTypes = namedTypes
	}
}

// WithPackageName sets the package name of the generated Go source.
func WithPackageName(packageName string) GeneratorOption {
	return func(g *Generator) {
		g.packageName = packageName
	}
}

// WithPreserveOrder sets whether to preserve the order of types and fields.
func WithPreserveOrder(preserveOrder bool) GeneratorOption {
	return func(g *Generator) {
		g.preserveOrder = preserveOrder
	}
}

// WithTimeLayout sets the time layout used to identify times in the observed
// XML documents. Use an empty string to disable identifying times.
func WithTimeLayout(timeLayout string) GeneratorOption {
	return func(g *Generator) {
		g.timeLayout = timeLayout
	}
}

// WithTopLevelAttributes sets whether to include top level attributes.
func WithTopLevelAttributes(topLevelAttributes bool) GeneratorOption {
	return func(g *Generator) {
		g.topLevelAttributes = topLevelAttributes
	}
}

// WithUsePointersForOptionFields sets whether to use pointers for optional
// fields in the generated Go source.
func WithUsePointersForOptionalFields(usePointersForOptionalFields bool) GeneratorOption {
	return func(g *Generator) {
		g.usePointersForOptionalFields = usePointersForOptionalFields
	}
}

// NewGenerator returns a new Generator with the given options.
func NewGenerator(options ...GeneratorOption) *Generator {
	generator := &Generator{
		charDataFieldName:            DefaultCharDataFieldName,
		exportNameFunc:               DefaultExportNameFunc,
		formatSource:                 DefaultFormatSource,
		header:                       DefaultHeader,
		intType:                      DefaultIntType,
		nameFunc:                     DefaultNameFunc,
		namedTypes:                   DefaultNamedTypes,
		packageName:                  DefaultPackageName,
		preserveOrder:                DefaultPreserveOrder,
		timeLayout:                   DefaultTimeLayout,
		topLevelAttributes:           DefaultTopLevelAttributes,
		typeOrder:                    make(map[xml.Name]int),
		usePointersForOptionalFields: DefaultUsePointersForOptionalFields,
		typeElements:                 make(map[xml.Name]*element),
	}
	for _, option := range options {
		option(generator)
	}
	return generator
}

// Generate returns the generated Go source for all the XML documents observed
// so far.
func (g *Generator) Generate() ([]byte, error) {
	options := generateOptions{
		charDataFieldName:            g.charDataFieldName,
		exportNameFunc:               g.exportNameFunc,
		header:                       g.header,
		importPackageNames:           make(map[string]struct{}),
		intType:                      g.intType,
		preserveOrder:                g.preserveOrder,
		usePointersForOptionalFields: g.usePointersForOptionalFields,
	}

	var typeElements []*element
	if g.namedTypes {
		options.namedTypes = maps.Clone(g.typeElements)
		options.simpleTypes = make(map[xml.Name]struct{})
		for name, element := range options.namedTypes {
			if len(element.attrValues) != 0 || len(element.childElements) != 0 {
				continue
			}
			options.simpleTypes[name] = struct{}{}
			delete(options.namedTypes, name)
		}
		typeElements = maps.Values(options.namedTypes)
	} else {
		typeElements = maps.Values(g.typeElements)
	}

	if options.preserveOrder {
		slices.SortFunc(typeElements, func(a, b *element) bool {
			return g.typeOrder[a.name] < g.typeOrder[b.name]
		})
	} else {
		slices.SortFunc(typeElements, func(a, b *element) bool {
			return options.exportNameFunc(a.name) < options.exportNameFunc(b.name)
		})
	}

	typesBuilder := &strings.Builder{}
	typeNames := make(map[string]struct{})
	for _, typeElement := range typeElements {
		typeName := options.exportNameFunc(typeElement.name)
		if _, ok := typeNames[typeName]; ok {
			return nil, fmt.Errorf("%s: duplicate type name", typeName)
		}
		typeNames[typeName] = struct{}{}
		fmt.Fprintf(typesBuilder, "\ntype %s ", typeName)
		if err := typeElement.writeGoType(typesBuilder, &options, ""); err != nil {
			return nil, err
		}
		typesBuilder.WriteByte('\n')
	}

	sourceBuilder := &strings.Builder{}
	if options.header != "" {
		fmt.Fprintf(sourceBuilder, "%s\n\n", options.header)
	}
	fmt.Fprintf(sourceBuilder, "package %s\n\n", g.packageName)
	switch len(options.importPackageNames) {
	case 0:
		// Do nothing.
	case 1:
		for importPackageName := range options.importPackageNames {
			fmt.Fprintf(sourceBuilder, "import %q\n", importPackageName)
		}
	default:
		fmt.Fprintf(sourceBuilder, "import (\n")
		importPackageNames := maps.Keys(options.importPackageNames)
		sort.Strings(importPackageNames)
		for _, importPackageName := range importPackageNames {
			fmt.Fprintf(sourceBuilder, "\t%q\n", importPackageName)
		}
		fmt.Fprintf(sourceBuilder, ")\n")
	}
	sourceBuilder.WriteString(typesBuilder.String())

	source := []byte(sourceBuilder.String())
	if !g.formatSource {
		return source, nil
	}
	return format.Source(source)
}

// ObserveFile observes an XML document in the given file.
func (g *Generator) ObserveFile(name string) error {
	file, err := os.Open(name)
	if err != nil {
		return err
	}
	defer file.Close()
	return g.ObserveReader(file)
}

// ObserveReader observes an XML document from r.
func (g *Generator) ObserveReader(r io.Reader) error {
	options := observeOptions{
		getOrder: func() int {
			g.order++
			return g.order
		},
		nameFunc:           g.nameFunc,
		timeLayout:         g.timeLayout,
		topLevelAttributes: g.topLevelAttributes,
		typeOrder:          g.typeOrder,
	}
	if g.namedTypes {
		options.topLevelElements = g.typeElements
	}

	decoder := xml.NewDecoder(r)
	decoder.CharsetReader = charset.NewReaderLabel
	for {
		switch token, err := decoder.Token(); {
		case errors.Is(err, io.EOF):
			return nil
		case err != nil:
			return err
		default:
			if startElement, ok := token.(xml.StartElement); ok {
				name := g.nameFunc(startElement.Name)
				typeElement, ok := g.typeElements[name]
				if !ok {
					typeElement = newElement(name)
					g.typeElements[name] = typeElement
				}
				if _, ok := g.typeOrder[name]; !ok {
					g.typeOrder[name] = options.getOrder()
				}
				if err := typeElement.observeChildElement(decoder, startElement, 0, &options); err != nil {
					return err
				}
			}
		}
	}
}
