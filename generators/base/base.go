package base

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"strings"

	bungen "github.com/LdDl/bungen/lib"
	"github.com/LdDl/bungen/model"
	"github.com/LdDl/bungen/util"

	"github.com/spf13/cobra"
)

const (
	// Conn is connection string (-c) basic flag
	Conn = "conn"

	// Output is output filename (-o) basic flag
	Output = "output"

	// Tables is basic flag (-t) for tables to generate
	Tables = "tables"

	// FollowFKs is basic flag (-f) for generate foreign keys models for selected tables
	FollowFKs = "follow-fk"

	// Package for model files
	Pkg = "pkg"

	// uuid type flag
	uuidFlag = "uuid"

	// custom types flag
	customTypesFlag = "custom-types"

	// generate simple ORM queries
	genORM = "gen-orm"
	dbWrap = "db-wrap"
)

// Gen is interface for all generators
type Gen interface {
	AddFlags(command *cobra.Command)
	ReadFlags(command *cobra.Command) error

	Generate() error
}

// Packer is a function that compile entities to package
type Packer func(entities []model.Entity) (interface{}, error)

// Options is common options for all generators
type Options struct {
	// URL connection string
	URL string

	// Output file path
	Output string

	// List of Tables to generate
	// Default []string{"public.*"}
	Tables []string

	// Generate model for foreign keys,
	// even if Tables not listed in Tables param
	// will not generate fks if schema not listed
	FollowFKs bool

	// Custom types goes here
	CustomTypes model.CustomTypeMapping

	// Generate basic ORM queries
	GenORM bool

	// Struct name for ORM queries. Works only when GenORM == true
	DBWrapName string
}

// Def sets default options if empty
func (o *Options) Def() {
	if len(o.Tables) == 0 {
		o.Tables = []string{util.Join(util.PublicSchema, "*")}
	}

	if o.CustomTypes == nil {
		o.CustomTypes = model.CustomTypeMapping{}
	}
}

// Generator is base generator used in other generators
type Generator struct {
	bungen.Bungen
}

// NewGenerator creates generator
func NewGenerator(url string) Generator {
	return Generator{
		Bungen: bungen.New(url, nil),
	}
}

// AddFlags adds basic flags to command
func AddFlags(command *cobra.Command) {
	flags := command.Flags()

	flags.StringP(Conn, "c", "", "connection string to your postgres database")
	if err := command.MarkFlagRequired(Conn); err != nil {
		panic(err)
	}

	flags.StringP(Output, "o", "", "output file name")
	if err := command.MarkFlagRequired(Output); err != nil {
		panic(err)
	}

	flags.StringP(Pkg, "p", "", "package for model files. if not set last folder name in output path will be used")

	flags.StringSliceP(Tables, "t", []string{"public.*"}, "table names for model generation separated by comma\nuse 'schema_name.*' to generate model for every table in model")
	flags.BoolP(FollowFKs, "f", false, "generate models for foreign keys, even if it not listed in Tables\n")

	flags.Bool(uuidFlag, false, "use github.com/google/uuid as type for uuid")

	flags.StringSlice(customTypesFlag, []string{}, "set custom types separated by comma\nformat: <postgresql_type>:<go_import>.<go_type>\nexamples: uuid:github.com/google/uuid.UUID,point:src/model.Point,bytea:string\n")

	flags.BoolP(genORM, "q", false, "generate basic ORM queries")
	flags.StringP(dbWrap, "z", "DatabaseWrap", "name of structs for wrapping ORM queries (works only with flag -q, --gen-orm)")

	return
}

// ReadFlags reads basic flags from command
func ReadFlags(command *cobra.Command) (conn, output, pkg string, tables []string, followFKs bool, genOrm bool, dbWrapName string, customTypes model.CustomTypeMapping, err error) {
	var customTypesStrings []string
	uuid := false

	flags := command.Flags()

	if conn, err = flags.GetString(Conn); err != nil {
		return
	}

	if output, err = flags.GetString(Output); err != nil {
		return
	}

	if pkg, err = flags.GetString(Pkg); err != nil {
		return
	}

	if strings.Trim(pkg, " ") == "" {
		pkg = util.PackageFromOutput(output)
	}

	if tables, err = flags.GetStringSlice(Tables); err != nil {
		return
	}

	if followFKs, err = flags.GetBool(FollowFKs); err != nil {
		return
	}

	if genOrm, err = flags.GetBool(genORM); err != nil {
		return
	}
	if dbWrapName, err = flags.GetString(dbWrap); err != nil {
		return
	}

	if customTypesStrings, err = flags.GetStringSlice(customTypesFlag); err != nil {
		return
	}

	if customTypes, err = model.ParseCustomTypes(customTypesStrings); err != nil {
		return
	}
	if uuid, err = flags.GetBool(uuidFlag); err != nil {
		return
	}

	if uuid && !customTypes.Has(model.TypePGUuid) {
		customTypes.Add(model.TypePGUuid, "uuid.UUID", "github.com/google/uuid")
	}

	return
}

// Generate runs whole generation process
func (g Generator) Generate(tables []string, followFKs, useSQLNulls bool, output, tmpl string, packer Packer, customTypes model.CustomTypeMapping) error {
	entities, err := g.Read(tables, followFKs, useSQLNulls, customTypes)
	if err != nil {
		return fmt.Errorf("read database error: %w", err)
	}
	return g.GenerateFromEntities(entities, output, tmpl, packer)
}

func (g Generator) GenerateFromEntities(entities []model.Entity, output, tmpl string, packer Packer) error {
	parsed, err := template.New("base").Parse(tmpl)
	if err != nil {
		return fmt.Errorf("parsing template error: %w", err)
	}

	pack, err := packer(entities)
	if err != nil {
		return fmt.Errorf("packing data error: %w", err)
	}

	var buffer bytes.Buffer
	if err := parsed.ExecuteTemplate(&buffer, "base", pack); err != nil {
		return fmt.Errorf("processing model template error: %w", err)
	}

	saved, err := util.FmtAndSave(buffer.Bytes(), output)
	if err != nil {
		if !saved {
			return fmt.Errorf("saving file error: %w", err)
		}
		return fmt.Errorf("formatting file %s error: %w (unformatted file kept for inspection)", output, err)
	}

	log.Printf("successfully generated %d models", len(entities))

	return nil
}

// ReadTemplate returns the content of the template file at path,
// or def when path is empty
func ReadTemplate(path, def string) (string, error) {
	if path == "" {
		return def, nil
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading template error: %w", err)
	}

	return string(content), nil
}

// CreateCommand creates cobra command
func CreateCommand(name, description string, generator Gen) *cobra.Command {
	command := &cobra.Command{
		Use:   name,
		Short: description,
		Long:  "",
		Run: func(command *cobra.Command, args []string) {
			if !command.HasFlags() {
				if err := command.Help(); err != nil {
					log.Printf("help not found, error: %s", err)
				}
				os.Exit(0)
				return
			}

			if err := generator.ReadFlags(command); err != nil {
				log.Printf("read flags error: %s", err)
				os.Exit(1)
			}

			if err := generator.Generate(); err != nil {
				log.Printf("generate error: %s", err)
				os.Exit(1)
			}
		},
		FParseErrWhitelist: cobra.FParseErrWhitelist{
			UnknownFlags: true,
		},
	}

	generator.AddFlags(command)

	return command
}
