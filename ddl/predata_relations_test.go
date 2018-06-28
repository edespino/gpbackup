package ddl_test

import (
	"database/sql"
	"sort"

	"github.com/greenplum-db/gp-common-go-libs/structmatcher"
	"github.com/greenplum-db/gp-common-go-libs/testhelper"
	"github.com/greenplum-db/gpbackup/ddl"
	"github.com/greenplum-db/gpbackup/testutils"

	"math"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("backup/predata_relations tests", func() {
	testTable := ddl.Relation{Schema: "public", Name: "tablename"}

	distRandom := "DISTRIBUTED RANDOMLY"
	distSingle := "DISTRIBUTED BY (i)"
	distComposite := "DISTRIBUTED BY (i, j)"

	rowOne := ddl.ColumnDefinition{Oid: 0, Num: 1, Name: "i", Type: "integer", StatTarget: -1, ACL: []ddl.ACL{}}
	rowTwo := ddl.ColumnDefinition{Oid: 1, Num: 2, Name: "j", Type: "character varying(20)", StatTarget: -1, ACL: []ddl.ACL{}}

	heapOpts := ""
	aoOpts := "appendonly=true"
	coOpts := "appendonly=true, orientation=column"
	heapFillOpts := "fillfactor=42"
	coManyOpts := "appendonly=true, orientation=column, fillfactor=42, compresstype=zlib, blocksize=32768, compresslevel=1"

	partDefEmpty := ""
	partTemplateDefEmpty := ""
	colDefsEmpty := []ddl.ColumnDefinition{}
	extTableEmpty := ddl.ExternalTableDefinition{Oid: 0, Type: -2, Protocol: -2, Location: "", ExecLocation: "ALL_SEGMENTS", FormatType: "t", FormatOpts: "", Options: "", Command: "", RejectLimit: 0, RejectLimitType: "", ErrTableName: "", ErrTableSchema: "", Encoding: "UTF-8", Writable: false, URIs: nil}

	partDef := `PARTITION BY LIST(gender)
	(
	PARTITION girls VALUES('F') WITH (tablename='rank_1_prt_girls', appendonly=false ),
	PARTITION boys VALUES('M') WITH (tablename='rank_1_prt_boys', appendonly=false ),
	DEFAULT PARTITION other  WITH (tablename='rank_1_prt_other', appendonly=false )
	)
`

	partTemplateDef := `ALTER TABLE tablename
SET SUBPARTITION TEMPLATE
          (
          SUBPARTITION usa VALUES('usa') WITH (tablename='tablename'),
          SUBPARTITION asia VALUES('asia') WITH (tablename='tablename'),
          SUBPARTITION europe VALUES('europe') WITH (tablename='tablename'),
          DEFAULT SUBPARTITION other_regions  WITH (tablename='tablename')
          )
`

	noMetadata := ddl.ObjectMetadata{}

	BeforeEach(func() {
		toc, backupfile = testutils.InitializeTestTOC(buffer, "predata")
	})
	Describe("PrintCreateTableStatement", func() {
		tableDef := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapOpts, ColumnDefs: colDefsEmpty, ExtTableDef: extTableEmpty}
		It("calls PrintRegularTableCreateStatement for a regular table", func() {
			tableMetadata := ddl.ObjectMetadata{Owner: "testrole"}

			tableDef.IsExternal = false
			ddl.PrintCreateTableStatement(backupfile, toc, testTable, tableDef, tableMetadata)
			testutils.ExpectEntry(toc.PredataEntries, 0, "public", "", "tablename", "TABLE")
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
) DISTRIBUTED RANDOMLY;


ALTER TABLE public.tablename OWNER TO testrole;`)
		})
		It("calls PrintExternalTableCreateStatement for an external table", func() {
			tableDef.IsExternal = true
			ddl.PrintCreateTableStatement(backupfile, toc, testTable, tableDef, noMetadata)
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE READABLE EXTERNAL WEB TABLE public.tablename (
) 
FORMAT 'text'
ENCODING 'UTF-8';`)
		})
	})
	Describe("PrintRegularTableCreateStatement", func() {
		rowOneEncoding := ddl.ColumnDefinition{Oid: 0, Num: 1, Name: "i", Type: "integer", Encoding: "compresstype=none,blocksize=32768,compresslevel=0", StatTarget: -1}
		rowTwoEncoding := ddl.ColumnDefinition{Oid: 0, Num: 2, Name: "j", Type: "character varying(20)", Encoding: "compresstype=zlib,blocksize=65536,compresslevel=1", StatTarget: -1}
		rowNotNull := ddl.ColumnDefinition{Oid: 0, Num: 2, Name: "j", NotNull: true, Type: "character varying(20)", StatTarget: -1}
		rowEncodingNotNull := ddl.ColumnDefinition{Oid: 0, Num: 2, Name: "j", NotNull: true, Type: "character varying(20)", Encoding: "compresstype=zlib,blocksize=65536,compresslevel=1", StatTarget: -1}
		rowOneDef := ddl.ColumnDefinition{Oid: 0, Num: 1, Name: "i", HasDefault: true, Type: "integer", StatTarget: -1, DefaultVal: "42"}
		rowTwoDef := ddl.ColumnDefinition{Oid: 0, Num: 2, Name: "j", HasDefault: true, Type: "character varying(20)", StatTarget: -1, DefaultVal: "'bar'::text"}
		rowTwoEncodingDef := ddl.ColumnDefinition{Oid: 0, Num: 2, Name: "j", HasDefault: true, Type: "character varying(20)", Encoding: "compresstype=zlib,blocksize=65536,compresslevel=1", StatTarget: -1, DefaultVal: "'bar'::text"}
		rowNotNullDef := ddl.ColumnDefinition{Oid: 0, Num: 2, Name: "j", NotNull: true, HasDefault: true, Type: "character varying(20)", StatTarget: -1, DefaultVal: "'bar'::text"}
		rowEncodingNotNullDef := ddl.ColumnDefinition{Oid: 0, Num: 2, Name: "j", NotNull: true, HasDefault: true, Type: "character varying(20)", Encoding: "compresstype=zlib,blocksize=65536,compresslevel=1", StatTarget: -1, DefaultVal: "'bar'::text"}
		rowStats := ddl.ColumnDefinition{Oid: 0, Num: 1, Name: "i", Type: "integer", StatTarget: 3}
		colOptions := ddl.ColumnDefinition{Oid: 0, Num: 1, Name: "i", Type: "integer", Options: "n_distinct=1", StatTarget: -1}
		colStorageType := ddl.ColumnDefinition{Oid: 0, Num: 1, Name: "i", Type: "integer", StatTarget: -1, StorageType: "PLAIN"}
		tableDefWithType := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapOpts, ExtTableDef: extTableEmpty, TableType: "public.some_type"}

		Context("No special table attributes", func() {
			tableDef := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapOpts, ExtTableDef: extTableEmpty}
			It("prints a CREATE TABLE OF type block with one line", func() {
				col := []ddl.ColumnDefinition{rowOne}
				tableDefWithType.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDefWithType)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename OF public.some_type (
	i WITH OPTIONS
) DISTRIBUTED RANDOMLY;`)
			})

			It("prints a CREATE TABLE block with one line", func() {
				col := []ddl.ColumnDefinition{rowOne}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer
) DISTRIBUTED RANDOMLY;`)
			})
			It("prints a CREATE TABLE block with one line per attribute", func() {
				col := []ddl.ColumnDefinition{rowOne, rowTwo}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) DISTRIBUTED RANDOMLY;`)
			})
			It("prints a CREATE TABLE block with no attributes", func() {
				tableDef.ColumnDefs = colDefsEmpty
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
) DISTRIBUTED RANDOMLY;`)
			})
		})
		Context("One special table attribute", func() {
			tableDef := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapOpts, ExtTableDef: extTableEmpty}
			It("prints a CREATE TABLE block where one line has the given ENCODING and the other has the default ENCODING", func() {
				col := []ddl.ColumnDefinition{rowOneEncoding, rowTwoEncoding}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer ENCODING (compresstype=none,blocksize=32768,compresslevel=0),
	j character varying(20) ENCODING (compresstype=zlib,blocksize=65536,compresslevel=1)
) DISTRIBUTED RANDOMLY;`)
			})
			It("prints a CREATE TABLE block where one line contains NOT NULL", func() {
				col := []ddl.ColumnDefinition{rowOne, rowNotNull}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20) NOT NULL
) DISTRIBUTED RANDOMLY;`)
			})
			It("prints a CREATE TABLE OF type block where one line contains NOT NULL", func() {
				col := []ddl.ColumnDefinition{rowOne, rowNotNull}
				tableDefWithType.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDefWithType)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename OF public.some_type (
	i WITH OPTIONS,
	j WITH OPTIONS NOT NULL
) DISTRIBUTED RANDOMLY;`)
			})
			It("prints a CREATE TABLE block where one line contains DEFAULT", func() {
				col := []ddl.ColumnDefinition{rowOneDef, rowTwo}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer DEFAULT 42,
	j character varying(20)
) DISTRIBUTED RANDOMLY;`)
			})
			It("prints a CREATE TABLE block where both lines contain DEFAULT", func() {
				col := []ddl.ColumnDefinition{rowOneDef, rowTwoDef}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer DEFAULT 42,
	j character varying(20) DEFAULT 'bar'::text
) DISTRIBUTED RANDOMLY;`)
			})
			It("prints a CREATE TABLE block followed by an ALTER COLUMN ... SET STATISTICS statement", func() {
				col := []ddl.ColumnDefinition{rowStats}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer
) DISTRIBUTED RANDOMLY;

ALTER TABLE ONLY public.tablename ALTER COLUMN i SET STATISTICS 3;`)
			})
			It("prints a CREATE TABLE block followed by an ALTER COLUMN ... SET STORAGE statement", func() {
				col := []ddl.ColumnDefinition{colStorageType}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer
) DISTRIBUTED RANDOMLY;

ALTER TABLE ONLY public.tablename ALTER COLUMN i SET STORAGE PLAIN;`)
			})
			It("prints a CREATE TABLE block followed by an ALTER COLUMN ... SET ... statement", func() {
				col := []ddl.ColumnDefinition{colOptions}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer
) DISTRIBUTED RANDOMLY;

ALTER TABLE ONLY public.tablename ALTER COLUMN i SET (n_distinct=1);`)
			})
		})
		Context("Multiple special table attributes on one column", func() {
			tableDef := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapOpts, ExtTableDef: extTableEmpty}
			It("prints a CREATE TABLE block where one line contains both NOT NULL and ENCODING", func() {
				col := []ddl.ColumnDefinition{rowOneEncoding, rowEncodingNotNull}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer ENCODING (compresstype=none,blocksize=32768,compresslevel=0),
	j character varying(20) NOT NULL ENCODING (compresstype=zlib,blocksize=65536,compresslevel=1)
) DISTRIBUTED RANDOMLY;`)
			})
			It("prints a CREATE TABLE block where one line contains both DEFAULT and NOT NULL", func() {
				col := []ddl.ColumnDefinition{rowOne, rowNotNullDef}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20) DEFAULT 'bar'::text NOT NULL
) DISTRIBUTED RANDOMLY;`)
			})
			It("prints a CREATE TABLE block where one line contains both DEFAULT and ENCODING", func() {
				col := []ddl.ColumnDefinition{rowOneEncoding, rowTwoEncodingDef}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer ENCODING (compresstype=none,blocksize=32768,compresslevel=0),
	j character varying(20) DEFAULT 'bar'::text ENCODING (compresstype=zlib,blocksize=65536,compresslevel=1)
) DISTRIBUTED RANDOMLY;`)
			})
			It("prints a CREATE TABLE block where one line contains all three of DEFAULT, NOT NULL, and ENCODING", func() {
				col := []ddl.ColumnDefinition{rowOneEncoding, rowEncodingNotNullDef}
				tableDef.ColumnDefs = col
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer ENCODING (compresstype=none,blocksize=32768,compresslevel=0),
	j character varying(20) DEFAULT 'bar'::text NOT NULL ENCODING (compresstype=zlib,blocksize=65536,compresslevel=1)
) DISTRIBUTED RANDOMLY;`)
			})
		})
		Context("Table qualities (distribution keys and storage options)", func() {
			col := []ddl.ColumnDefinition{rowOne, rowTwo}
			It("has a single-column distribution key", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distSingle, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) DISTRIBUTED BY (i);`)
			})
			It("has a multiple-column composite distribution key", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distComposite, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) DISTRIBUTED BY (i, j);`)
			})
			It("is an append-optimized table", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: aoOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (appendonly=true) DISTRIBUTED RANDOMLY;`)
			})
			It("is an append-optimized table with a single-column distribution key", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distSingle, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: aoOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (appendonly=true) DISTRIBUTED BY (i);`)
			})
			It("is an append-optimized table with a two-column composite distribution key", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distComposite, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: aoOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (appendonly=true) DISTRIBUTED BY (i, j);`)
			})
			It("is an append-optimized column-oriented table", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: coOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (appendonly=true, orientation=column) DISTRIBUTED RANDOMLY;`)
			})
			It("is an append-optimized column-oriented table with a single-column distribution key", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distSingle, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: coOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (appendonly=true, orientation=column) DISTRIBUTED BY (i);`)
			})
			It("is an append-optimized column-oriented table with a two-column composite distribution key", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distComposite, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: coOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (appendonly=true, orientation=column) DISTRIBUTED BY (i, j);`)
			})
			It("is a heap table with a fill factor", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapFillOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (fillfactor=42) DISTRIBUTED RANDOMLY;`)
			})
			It("is a heap table with a fill factor and a single-column distribution key", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distSingle, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapFillOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (fillfactor=42) DISTRIBUTED BY (i);`)
			})
			It("is a heap table with a fill factor and a multiple-column composite distribution key", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distComposite, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapFillOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (fillfactor=42) DISTRIBUTED BY (i, j);`)
			})
			It("is an append-optimized column-oriented table with complex storage options", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: coManyOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (appendonly=true, orientation=column, fillfactor=42, compresstype=zlib, blocksize=32768, compresslevel=1) DISTRIBUTED RANDOMLY;`)
			})
			It("is an append-optimized column-oriented table with complex storage options and a single-column distribution key", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distSingle, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: coManyOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (appendonly=true, orientation=column, fillfactor=42, compresstype=zlib, blocksize=32768, compresslevel=1) DISTRIBUTED BY (i);`)
			})
			It("is an append-optimized column-oriented table with complex storage options and a two-column composite distribution key", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distComposite, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: coManyOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (appendonly=true, orientation=column, fillfactor=42, compresstype=zlib, blocksize=32768, compresslevel=1) DISTRIBUTED BY (i, j);`)
			})
		})
		Context("Table partitioning", func() {
			col := []ddl.ColumnDefinition{rowOne, rowTwo}
			It("is a partition table with table attributes", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDef, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) DISTRIBUTED RANDOMLY PARTITION BY LIST(gender)
	(
	PARTITION girls VALUES('F') WITH (tablename='rank_1_prt_girls', appendonly=false ),
	PARTITION boys VALUES('M') WITH (tablename='rank_1_prt_boys', appendonly=false ),
	DEFAULT PARTITION other  WITH (tablename='rank_1_prt_other', appendonly=false )
	);`)
			})
			It("is a partition table with no table attributes", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDef, PartTemplateDef: partTemplateDefEmpty, StorageOpts: coOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) WITH (appendonly=true, orientation=column) DISTRIBUTED RANDOMLY PARTITION BY LIST(gender)
	(
	PARTITION girls VALUES('F') WITH (tablename='rank_1_prt_girls', appendonly=false ),
	PARTITION boys VALUES('M') WITH (tablename='rank_1_prt_boys', appendonly=false ),
	DEFAULT PARTITION other  WITH (tablename='rank_1_prt_other', appendonly=false )
	);`)
			})
			It("is a partition table with subpartitions and table attributes", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDef, PartTemplateDef: partTemplateDef, StorageOpts: heapOpts, ColumnDefs: col, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) DISTRIBUTED RANDOMLY PARTITION BY LIST(gender)
	(
	PARTITION girls VALUES('F') WITH (tablename='rank_1_prt_girls', appendonly=false ),
	PARTITION boys VALUES('M') WITH (tablename='rank_1_prt_boys', appendonly=false ),
	DEFAULT PARTITION other  WITH (tablename='rank_1_prt_other', appendonly=false )
	);
ALTER TABLE tablename
SET SUBPARTITION TEMPLATE
          (
          SUBPARTITION usa VALUES('usa') WITH (tablename='tablename'),
          SUBPARTITION asia VALUES('asia') WITH (tablename='tablename'),
          SUBPARTITION europe VALUES('europe') WITH (tablename='tablename'),
          DEFAULT SUBPARTITION other_regions  WITH (tablename='tablename')
          );`)
			})
		})
		Context("Tablespaces", func() {
			It("prints a CREATE TABLE block with a TABLESPACE clause", func() {
				tableDef := ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapOpts, TablespaceName: "test_tablespace", ColumnDefs: colDefsEmpty, ExtTableDef: extTableEmpty}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
) TABLESPACE test_tablespace DISTRIBUTED RANDOMLY;`)
			})
		})
		Context("Inheritance", func() {
			tableDef := ddl.TableDefinition{}
			BeforeEach(func() {
				tableDef = ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapOpts, ExtTableDef: extTableEmpty}
			})
			AfterEach(func() {
				testTable.DependsUpon = []string{}
				testTable.Inherits = []string{}
			})
			It("prints a CREATE TABLE block with a single-inheritance INHERITS clause", func() {
				col := []ddl.ColumnDefinition{rowOne}
				tableDef.ColumnDefs = col
				testTable.DependsUpon = []string{"public.parent"}
				testTable.Inherits = []string{"public.parent"}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer
) INHERITS (public.parent) DISTRIBUTED RANDOMLY;`)
			})
			It("prints a CREATE TABLE block with a multiple-inheritance INHERITS clause", func() {
				col := []ddl.ColumnDefinition{rowOne, rowTwo}
				tableDef.ColumnDefs = col
				testTable.DependsUpon = []string{"public.parent_one", "public.parent_two"}
				testTable.Inherits = []string{"public.parent_one", "public.parent_two"}
				ddl.PrintRegularTableCreateStatement(backupfile, toc, testTable, tableDef)
				testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE TABLE public.tablename (
	i integer,
	j character varying(20)
) INHERITS (public.parent_one, public.parent_two) DISTRIBUTED RANDOMLY;`)
			})
		})
	})
	Describe("PrintPostCreateTableStatements", func() {
		rowCommentOne := ddl.ColumnDefinition{Oid: 0, Num: 1, Name: "i", Type: "integer", StatTarget: -1, Comment: "This is a column comment.", ACL: []ddl.ACL{}}
		rowCommentTwo := ddl.ColumnDefinition{Oid: 0, Num: 2, Name: "j", Type: "integer", StatTarget: -1, Comment: "This is another column comment.", ACL: []ddl.ACL{}}
		testTable := ddl.Relation{Schema: "public", Name: "tablename"}
		tableDef := ddl.TableDefinition{}
		BeforeEach(func() {
			tableDef = ddl.TableDefinition{DistPolicy: distRandom, PartDef: partDefEmpty, PartTemplateDef: partTemplateDefEmpty, StorageOpts: heapOpts, ExtTableDef: extTableEmpty}
		})

		It("prints a block with a table comment", func() {
			col := []ddl.ColumnDefinition{rowOne}
			tableDef.ColumnDefs = col
			tableMetadata := ddl.ObjectMetadata{Comment: "This is a table comment."}
			ddl.PrintPostCreateTableStatements(backupfile, testTable, tableDef, tableMetadata)
			testhelper.ExpectRegexp(buffer, `

COMMENT ON TABLE public.tablename IS 'This is a table comment.';`)
		})
		It("prints a block with a single column comment", func() {
			col := []ddl.ColumnDefinition{rowCommentOne}
			tableDef.ColumnDefs = col
			ddl.PrintPostCreateTableStatements(backupfile, testTable, tableDef, noMetadata)
			testhelper.ExpectRegexp(buffer, `

COMMENT ON COLUMN public.tablename.i IS 'This is a column comment.';`)
		})
		It("prints a block with a single column comment containing special characters", func() {
			rowCommentSpecialCharacters := ddl.ColumnDefinition{Oid: 0, Num: 1, Name: "i", Type: "integer", StatTarget: -1, Comment: `This is a ta'ble 1+=;,./\>,<@\\n^comment.`, ACL: []ddl.ACL{}}

			col := []ddl.ColumnDefinition{rowCommentSpecialCharacters}
			tableDef.ColumnDefs = col
			ddl.PrintPostCreateTableStatements(backupfile, testTable, tableDef, noMetadata)
			testhelper.ExpectRegexp(buffer, `

COMMENT ON COLUMN public.tablename.i IS 'This is a ta''ble 1+=;,./\>,<@\\n^comment.';`)
		})
		It("prints a block with multiple column comments", func() {
			col := []ddl.ColumnDefinition{rowCommentOne, rowCommentTwo}
			tableDef.ColumnDefs = col
			ddl.PrintPostCreateTableStatements(backupfile, testTable, tableDef, noMetadata)
			testhelper.ExpectRegexp(buffer, `

COMMENT ON COLUMN public.tablename.i IS 'This is a column comment.';


COMMENT ON COLUMN public.tablename.j IS 'This is another column comment.';`)
		})
		It("prints an ALTER TABLE ... OWNER TO statement to set the table owner", func() {
			col := []ddl.ColumnDefinition{rowOne}
			tableDef.ColumnDefs = col
			tableMetadata := ddl.ObjectMetadata{Owner: "testrole"}
			ddl.PrintPostCreateTableStatements(backupfile, testTable, tableDef, tableMetadata)
			testhelper.ExpectRegexp(buffer, `

ALTER TABLE public.tablename OWNER TO testrole;`)
		})
		It("prints both an ALTER TABLE ... OWNER TO statement and comments", func() {
			col := []ddl.ColumnDefinition{rowCommentOne, rowCommentTwo}
			tableDef.ColumnDefs = col
			tableMetadata := ddl.ObjectMetadata{Owner: "testrole", Comment: "This is a table comment."}
			ddl.PrintPostCreateTableStatements(backupfile, testTable, tableDef, tableMetadata)
			testhelper.ExpectRegexp(buffer, `

COMMENT ON TABLE public.tablename IS 'This is a table comment.';


ALTER TABLE public.tablename OWNER TO testrole;


COMMENT ON COLUMN public.tablename.i IS 'This is a column comment.';


COMMENT ON COLUMN public.tablename.j IS 'This is another column comment.';`)
		})
		It("prints a GRANT statement on a table column", func() {
			privilegesColumnOne := ddl.ColumnDefinition{Oid: 0, Num: 1, Name: "i", Type: "integer", StatTarget: -1, ACL: []ddl.ACL{{Grantee: "testrole", Select: true}}}
			privilegesColumnTwo := ddl.ColumnDefinition{Oid: 1, Num: 2, Name: "j", Type: "character varying(20)", StatTarget: -1, ACL: []ddl.ACL{{Grantee: "testrole2", Select: true, Insert: true, Update: true, References: true}}}
			col := []ddl.ColumnDefinition{privilegesColumnOne, privilegesColumnTwo}
			tableDef.ColumnDefs = col
			tableMetadata := ddl.ObjectMetadata{Owner: "testrole"}
			ddl.PrintPostCreateTableStatements(backupfile, testTable, tableDef, tableMetadata)
			testhelper.ExpectRegexp(buffer, `

ALTER TABLE public.tablename OWNER TO testrole;


REVOKE ALL (i) ON TABLE public.tablename FROM PUBLIC;
REVOKE ALL (i) ON TABLE public.tablename FROM testrole;
GRANT SELECT (i) ON TABLE public.tablename TO testrole;


REVOKE ALL (j) ON TABLE public.tablename FROM PUBLIC;
REVOKE ALL (j) ON TABLE public.tablename FROM testrole;
GRANT ALL (j) ON TABLE public.tablename TO testrole2;`)
		})
	})
	Describe("PrintCreateSequenceStatements", func() {
		baseSequence := ddl.Relation{SchemaOid: 0, Oid: 1, Schema: "public", Name: "seq_name", DependsUpon: nil, Inherits: nil}
		seqDefault := ddl.Sequence{Relation: baseSequence, SequenceDefinition: ddl.SequenceDefinition{Name: "seq_name", LastVal: 7, Increment: 1, MaxVal: math.MaxInt64, MinVal: 1, CacheVal: 5, LogCnt: 42, IsCycled: false, IsCalled: true}}
		seqNegIncr := ddl.Sequence{Relation: baseSequence, SequenceDefinition: ddl.SequenceDefinition{Name: "seq_name", LastVal: 7, Increment: -1, MaxVal: -1, MinVal: math.MinInt64, CacheVal: 5, LogCnt: 42, IsCycled: false, IsCalled: true}}
		seqMaxPos := ddl.Sequence{Relation: baseSequence, SequenceDefinition: ddl.SequenceDefinition{Name: "seq_name", LastVal: 7, Increment: 1, MaxVal: 100, MinVal: 1, CacheVal: 5, LogCnt: 42, IsCycled: false, IsCalled: true}}
		seqMinPos := ddl.Sequence{Relation: baseSequence, SequenceDefinition: ddl.SequenceDefinition{Name: "seq_name", LastVal: 7, Increment: 1, MaxVal: math.MaxInt64, MinVal: 10, CacheVal: 5, LogCnt: 42, IsCycled: false, IsCalled: true}}
		seqMaxNeg := ddl.Sequence{Relation: baseSequence, SequenceDefinition: ddl.SequenceDefinition{Name: "seq_name", LastVal: 7, Increment: -1, MaxVal: -10, MinVal: math.MinInt64, CacheVal: 5, LogCnt: 42, IsCycled: false, IsCalled: true}}
		seqMinNeg := ddl.Sequence{Relation: baseSequence, SequenceDefinition: ddl.SequenceDefinition{Name: "seq_name", LastVal: 7, Increment: -1, MaxVal: -1, MinVal: -100, CacheVal: 5, LogCnt: 42, IsCycled: false, IsCalled: true}}
		seqCycle := ddl.Sequence{Relation: baseSequence, SequenceDefinition: ddl.SequenceDefinition{Name: "seq_name", LastVal: 7, Increment: 1, MaxVal: math.MaxInt64, MinVal: 1, CacheVal: 5, LogCnt: 42, IsCycled: true, IsCalled: true}}
		seqStart := ddl.Sequence{Relation: baseSequence, SequenceDefinition: ddl.SequenceDefinition{Name: "seq_name", LastVal: 7, Increment: 1, MaxVal: math.MaxInt64, MinVal: 1, CacheVal: 5, LogCnt: 42, IsCycled: false, IsCalled: false}}
		emptySequenceMetadataMap := ddl.MetadataMap{}

		It("can print a sequence with all default options", func() {
			sequences := []ddl.Sequence{seqDefault}
			ddl.PrintCreateSequenceStatements(backupfile, toc, sequences, emptySequenceMetadataMap)
			testutils.ExpectEntry(toc.PredataEntries, 0, "public", "", "seq_name", "SEQUENCE")
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE SEQUENCE public.seq_name
	INCREMENT BY 1
	NO MAXVALUE
	NO MINVALUE
	CACHE 5;

SELECT pg_catalog.setval('public.seq_name', 7, true);`)
		})
		It("can print a decreasing sequence", func() {
			sequences := []ddl.Sequence{seqNegIncr}
			ddl.PrintCreateSequenceStatements(backupfile, toc, sequences, emptySequenceMetadataMap)
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE SEQUENCE public.seq_name
	INCREMENT BY -1
	NO MAXVALUE
	NO MINVALUE
	CACHE 5;

SELECT pg_catalog.setval('public.seq_name', 7, true);`)
		})
		It("can print an increasing sequence with a maximum value", func() {
			sequences := []ddl.Sequence{seqMaxPos}
			ddl.PrintCreateSequenceStatements(backupfile, toc, sequences, emptySequenceMetadataMap)
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE SEQUENCE public.seq_name
	INCREMENT BY 1
	MAXVALUE 100
	NO MINVALUE
	CACHE 5;

SELECT pg_catalog.setval('public.seq_name', 7, true);`)
		})
		It("can print an increasing sequence with a minimum value", func() {
			sequences := []ddl.Sequence{seqMinPos}
			ddl.PrintCreateSequenceStatements(backupfile, toc, sequences, emptySequenceMetadataMap)
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE SEQUENCE public.seq_name
	INCREMENT BY 1
	NO MAXVALUE
	MINVALUE 10
	CACHE 5;

SELECT pg_catalog.setval('public.seq_name', 7, true);`)
		})
		It("can print a decreasing sequence with a maximum value", func() {
			sequences := []ddl.Sequence{seqMaxNeg}
			ddl.PrintCreateSequenceStatements(backupfile, toc, sequences, emptySequenceMetadataMap)
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE SEQUENCE public.seq_name
	INCREMENT BY -1
	MAXVALUE -10
	NO MINVALUE
	CACHE 5;

SELECT pg_catalog.setval('public.seq_name', 7, true);`)
		})
		It("can print a decreasing sequence with a minimum value", func() {
			sequences := []ddl.Sequence{seqMinNeg}
			ddl.PrintCreateSequenceStatements(backupfile, toc, sequences, emptySequenceMetadataMap)
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE SEQUENCE public.seq_name
	INCREMENT BY -1
	NO MAXVALUE
	MINVALUE -100
	CACHE 5;

SELECT pg_catalog.setval('public.seq_name', 7, true);`)
		})
		It("can print a sequence that cycles", func() {
			sequences := []ddl.Sequence{seqCycle}
			ddl.PrintCreateSequenceStatements(backupfile, toc, sequences, emptySequenceMetadataMap)
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE SEQUENCE public.seq_name
	INCREMENT BY 1
	NO MAXVALUE
	NO MINVALUE
	CACHE 5
	CYCLE;

SELECT pg_catalog.setval('public.seq_name', 7, true);`)
		})
		It("can print a sequence with a start value", func() {
			sequences := []ddl.Sequence{seqStart}
			ddl.PrintCreateSequenceStatements(backupfile, toc, sequences, emptySequenceMetadataMap)
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE SEQUENCE public.seq_name
	START WITH 7
	INCREMENT BY 1
	NO MAXVALUE
	NO MINVALUE
	CACHE 5;

SELECT pg_catalog.setval('public.seq_name', 7, false);`)
		})
		It("can print a sequence with privileges, an owner, and a comment", func() {
			sequenceMetadataMap := testutils.DefaultMetadataMap("SEQUENCE", true, true, true)
			sequenceMetadata := sequenceMetadataMap[1]
			sequenceMetadata.Privileges[0].Update = false
			sequenceMetadataMap[1] = sequenceMetadata
			sequences := []ddl.Sequence{seqDefault}
			ddl.PrintCreateSequenceStatements(backupfile, toc, sequences, sequenceMetadataMap)
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE SEQUENCE public.seq_name
	INCREMENT BY 1
	NO MAXVALUE
	NO MINVALUE
	CACHE 5;

SELECT pg_catalog.setval('public.seq_name', 7, true);


COMMENT ON SEQUENCE public.seq_name IS 'This is a sequence comment.';


ALTER TABLE public.seq_name OWNER TO testrole;


REVOKE ALL ON SEQUENCE public.seq_name FROM PUBLIC;
REVOKE ALL ON SEQUENCE public.seq_name FROM testrole;
GRANT SELECT,USAGE ON SEQUENCE public.seq_name TO testrole;`)
		})
		It("can print a sequence with privileges WITH GRANT OPTION", func() {
			sequenceMetadataMap := ddl.MetadataMap{
				1: {Privileges: []ddl.ACL{testutils.DefaultACLWithGrantWithout("testrole", "SEQUENCE", "UPDATE")}}}
			sequenceMetadata := sequenceMetadataMap[1]
			sequenceMetadata.Privileges[0].Update = false
			sequenceMetadataMap[1] = sequenceMetadata
			sequences := []ddl.Sequence{seqDefault}
			ddl.PrintCreateSequenceStatements(backupfile, toc, sequences, sequenceMetadataMap)
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `CREATE SEQUENCE public.seq_name
	INCREMENT BY 1
	NO MAXVALUE
	NO MINVALUE
	CACHE 5;

SELECT pg_catalog.setval('public.seq_name', 7, true);


REVOKE ALL ON SEQUENCE public.seq_name FROM PUBLIC;
GRANT SELECT,USAGE ON SEQUENCE public.seq_name TO testrole WITH GRANT OPTION;`)
		})
	})
	Describe("PrintCreateViewStatements", func() {
		It("can print a basic view", func() {
			viewOne := ddl.View{Oid: 0, Schema: "public", Name: `"WowZa"`, Definition: "SELECT rolname FROM pg_role;", DependsUpon: []string{}}
			viewTwo := ddl.View{Oid: 1, Schema: "shamwow", Name: "shazam", Definition: "SELECT count(*) FROM pg_tables;", DependsUpon: []string{}}
			viewMetadataMap := ddl.MetadataMap{}
			ddl.PrintCreateViewStatements(backupfile, toc, []ddl.View{viewOne, viewTwo}, viewMetadataMap)
			testutils.ExpectEntry(toc.PredataEntries, 0, "public", "", `"WowZa"`, "VIEW")
			testutils.AssertBufferContents(toc.PredataEntries, buffer,
				`CREATE VIEW public."WowZa" AS SELECT rolname FROM pg_role;`,
				`CREATE VIEW shamwow.shazam AS SELECT count(*) FROM pg_tables;`)
		})
		It("can print a view with privileges, an owner, and a comment", func() {
			viewOne := ddl.View{Oid: 0, Schema: "public", Name: `"WowZa"`, Definition: "SELECT rolname FROM pg_role;", DependsUpon: []string{}}
			viewTwo := ddl.View{Oid: 1, Schema: "shamwow", Name: "shazam", Definition: "SELECT count(*) FROM pg_tables;", DependsUpon: []string{}}
			viewMetadataMap := testutils.DefaultMetadataMap("VIEW", true, true, true)
			ddl.PrintCreateViewStatements(backupfile, toc, []ddl.View{viewOne, viewTwo}, viewMetadataMap)
			testutils.AssertBufferContents(toc.PredataEntries, buffer,
				`CREATE VIEW public."WowZa" AS SELECT rolname FROM pg_role;`,
				`CREATE VIEW shamwow.shazam AS SELECT count(*) FROM pg_tables;


COMMENT ON VIEW shamwow.shazam IS 'This is a view comment.';


REVOKE ALL ON shamwow.shazam FROM PUBLIC;
REVOKE ALL ON shamwow.shazam FROM testrole;
GRANT ALL ON shamwow.shazam TO testrole;`)
		})
	})
	Describe("PrintAlterSequenceStatements", func() {
		baseSequence := ddl.Relation{Schema: "public", Name: "seq_name"}
		seqDefault := ddl.Sequence{Relation: baseSequence, SequenceDefinition: ddl.SequenceDefinition{Name: "seq_name", LastVal: 7, Increment: 1, MaxVal: math.MaxInt64, MinVal: 1, CacheVal: 5, LogCnt: 42, IsCycled: false, IsCalled: true}}
		emptyColumnOwnerMap := make(map[string]string, 0)
		It("prints nothing for a sequence without an owning column", func() {
			sequences := []ddl.Sequence{seqDefault}
			ddl.PrintAlterSequenceStatements(backupfile, toc, sequences, emptyColumnOwnerMap)
			Expect(len(toc.PredataEntries)).To(Equal(0))
			testhelper.NotExpectRegexp(buffer, `ALTER SEQUENCE`)
		})
		It("does not write an alter sequence statement for a sequence that is not in the backup", func() {
			columnOwnerMap := map[string]string{"public.seq_name2": "public.tablename.col_one"}
			sequences := []ddl.Sequence{seqDefault}
			ddl.PrintAlterSequenceStatements(backupfile, toc, sequences, columnOwnerMap)
			Expect(len(toc.PredataEntries)).To(Equal(0))
			testhelper.NotExpectRegexp(buffer, `ALTER SEQUENCE`)
		})
		It("can print an ALTER SEQUENCE statement for a sequence with an owning column", func() {
			columnOwnerMap := map[string]string{"public.seq_name": "public.tablename.col_one"}
			sequences := []ddl.Sequence{seqDefault}
			ddl.PrintAlterSequenceStatements(backupfile, toc, sequences, columnOwnerMap)
			testutils.ExpectEntry(toc.PredataEntries, 0, "public", "", "seq_name", "SEQUENCE OWNER")
			testutils.AssertBufferContents(toc.PredataEntries, buffer, `ALTER SEQUENCE public.seq_name OWNED BY public.tablename.col_one;`)
		})
	})
	Describe("SplitTablesByPartitionType", func() {
		var tables []ddl.Relation
		var tableDefs map[uint32]ddl.TableDefinition
		var includeList []string
		var expectedMetadataTables = []ddl.Relation{
			{Oid: 1, Schema: "public", Name: "part_parent1"},
			{Oid: 2, Schema: "public", Name: "part_parent2"},
			{Oid: 8, Schema: "public", Name: "test_table"},
		}
		BeforeEach(func() {
			tables = []ddl.Relation{
				{Oid: 1, Schema: "public", Name: "part_parent1"},
				{Oid: 2, Schema: "public", Name: "part_parent2"},
				{Oid: 3, Schema: "public", Name: "part_parent1_inter1"},
				{Oid: 4, Schema: "public", Name: "part_parent1_child1"},
				{Oid: 5, Schema: "public", Name: "part_parent1_child2"},
				{Oid: 6, Schema: "public", Name: "part_parent2_child1"},
				{Oid: 7, Schema: "public", Name: "part_parent2_child2"},
				{Oid: 8, Schema: "public", Name: "test_table"},
			}
			tableDefs = map[uint32]ddl.TableDefinition{
				1: {PartitionType: "p"},
				2: {PartitionType: "p"},
				3: {PartitionType: "i"},
				4: {PartitionType: "l"},
				5: {PartitionType: "l"},
				6: {PartitionType: "l"},
				7: {PartitionType: "l"},
				8: {PartitionType: "n"},
			}
		})
		Context("leafPartitionData and includeTables", func() {
			It("gets only parent partitions of included tables for metadata and only child partitions for data", func() {
				includeList = []string{"public.part_parent1", "public.part_parent2_child1", "public.part_parent2_child2", "public.test_table"}
				ddl.SetLeafPartitionData(true)
				defer ddl.SetLeafPartitionData(false)

				metadataTables, dataTables := ddl.SplitTablesByPartitionType(tables, tableDefs, includeList)

				Expect(metadataTables).To(Equal(expectedMetadataTables))

				expectedDataTables := []string{"public.part_parent1_child1", "public.part_parent1_child2", "public.part_parent2_child1", "public.part_parent2_child2", "public.test_table"}
				dataTableNames := make([]string, 0)
				for _, table := range dataTables {
					dataTableNames = append(dataTableNames, table.FQN())
				}
				sort.Strings(dataTableNames)

				Expect(len(dataTables)).To(Equal(5))
				Expect(dataTableNames).To(Equal(expectedDataTables))
			})
		})
		Context("leafPartitionData only", func() {
			It("gets only parent partitions for metadata and only child partitions in data", func() {
				ddl.SetLeafPartitionData(true)
				defer ddl.SetLeafPartitionData(false)
				includeList = []string{}
				metadataTables, dataTables := ddl.SplitTablesByPartitionType(tables, tableDefs, includeList)

				Expect(metadataTables).To(Equal(expectedMetadataTables))

				expectedDataTables := []string{"public.part_parent1_child1", "public.part_parent1_child2", "public.part_parent2_child1", "public.part_parent2_child2", "public.test_table"}
				dataTableNames := make([]string, 0)
				for _, table := range dataTables {
					dataTableNames = append(dataTableNames, table.FQN())
				}
				sort.Strings(dataTableNames)

				Expect(len(dataTables)).To(Equal(5))
				Expect(dataTableNames).To(Equal(expectedDataTables))
			})
		})
		Context("includeTables only", func() {
			It("gets only parent partitions of included tables for metadata and only included tables for data", func() {
				ddl.SetLeafPartitionData(false)
				includeList = []string{"public.part_parent1", "public.part_parent2_child1", "public.part_parent2_child2", "public.test_table"}
				metadataTables, dataTables := ddl.SplitTablesByPartitionType(tables, tableDefs, includeList)

				Expect(metadataTables).To(Equal(expectedMetadataTables))

				expectedDataTables := []string{"public.part_parent1", "public.part_parent2_child1", "public.part_parent2_child2", "public.test_table"}
				dataTableNames := make([]string, 0)
				for _, table := range dataTables {
					dataTableNames = append(dataTableNames, table.FQN())
				}
				sort.Strings(dataTableNames)

				Expect(len(dataTables)).To(Equal(4))
				Expect(dataTableNames).To(Equal(expectedDataTables))
			})
		})
		Context("neither leafPartitionData nor includeTables", func() {
			It("gets the same table list for both metadata and data", func() {
				includeList = []string{}
				tables = []ddl.Relation{
					{Oid: 1, Schema: "public", Name: "part_parent1"},
					{Oid: 2, Schema: "public", Name: "part_parent2"},
					{Oid: 8, Schema: "public", Name: "test_table"},
				}
				tableDefs = map[uint32]ddl.TableDefinition{
					1: {PartitionType: "p"},
					2: {PartitionType: "p"},
					8: {PartitionType: "n"},
				}
				ddl.SetLeafPartitionData(false)
				ddl.SetIncludeRelations([]string{})
				metadataTables, dataTables := ddl.SplitTablesByPartitionType(tables, tableDefs, includeList)

				Expect(metadataTables).To(Equal(expectedMetadataTables))

				expectedDataTables := []string{"public.part_parent1", "public.part_parent2", "public.test_table"}
				dataTableNames := make([]string, 0)
				for _, table := range dataTables {
					dataTableNames = append(dataTableNames, table.FQN())
				}
				sort.Strings(dataTableNames)

				Expect(len(dataTables)).To(Equal(3))
				Expect(dataTableNames).To(Equal(expectedDataTables))
			})
			It("adds a suffix to external partition tables", func() {
				includeList = []string{}
				tables = []ddl.Relation{
					{Oid: 1, Schema: "public", Name: "part_parent1_prt_1"},
					{Oid: 2, Schema: "public", Name: "long_naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaame"},
				}
				tableDefs = map[uint32]ddl.TableDefinition{
					1: {PartitionType: "l", IsExternal: true},
					2: {PartitionType: "l", IsExternal: true},
				}
				ddl.SetLeafPartitionData(false)
				ddl.SetIncludeRelations([]string{})
				metadataTables, _ := ddl.SplitTablesByPartitionType(tables, tableDefs, includeList)

				expectedTables := []ddl.Relation{
					{Oid: 1, Schema: "public", Name: "part_parent1_prt_1_ext_part_"},
					{Oid: 2, Schema: "public", Name: "long_naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa_ext_part_"},
				}
				Expect(len(metadataTables)).To(Equal(2))
				structmatcher.ExpectStructsToMatch(&expectedTables[0], &metadataTables[0])
				structmatcher.ExpectStructsToMatch(&expectedTables[1], &metadataTables[1])
			})
		})
	})
	Describe("AppendExtPartSuffix", func() {
		It("adds a suffix to an unquoted external partition table", func() {
			tablename := "name"
			expectedName := "name_ext_part_"
			suffixName := ddl.AppendExtPartSuffix(tablename)
			Expect(suffixName).To(Equal(expectedName))
		})
		It("adds a suffix to an unquoted external partition table that is too long", func() {
			tablename := "long_naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaame"
			expectedName := "long_naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa_ext_part_"
			suffixName := ddl.AppendExtPartSuffix(tablename)
			Expect(suffixName).To(Equal(expectedName))
		})
		It("adds a suffix to a quoted external partition table", func() {
			tablename := `"!name"`
			expectedName := `"!name_ext_part_"`
			suffixName := ddl.AppendExtPartSuffix(tablename)
			Expect(suffixName).To(Equal(expectedName))
		})
		It("adds a suffix to a quoted external partition table that is too long", func() {
			tablename := `"long!naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaame"`
			expectedName := `"long!naaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa_ext_part_"`
			suffixName := ddl.AppendExtPartSuffix(tablename)
			Expect(suffixName).To(Equal(expectedName))
		})
	})
	Describe("ExpandIncludeRelations", func() {
		testTables := []ddl.Relation{{Schema: "testschema", Name: "foo1"}, {Schema: "testschema", Name: "foo2"}}
		It("returns an empty slice if no includeRelations were specified", func() {
			resultIncludeRelations := ddl.ExpandIncludeRelations([]string{}, testTables)
			Expect(len(resultIncludeRelations)).To(Equal(0))
		})
		It("returns original include list if the new tables list is a subset of existing list", func() {
			resultIncludeRelations := ddl.ExpandIncludeRelations([]string{"testschema.foo1", "testschema.foo2", "testschema.foo3"}, testTables)
			sort.Strings(resultIncludeRelations)
			Expect(len(resultIncludeRelations)).To(Equal(3))
			Expect(resultIncludeRelations).To(Equal([]string{"testschema.foo1", "testschema.foo2", "testschema.foo3"}))
		})
		It("returns expanded include list if there are new tables to add", func() {
			resultIncludeRelations := ddl.ExpandIncludeRelations([]string{"testschema.foo2", "testschema.foo3"}, testTables)
			sort.Strings(resultIncludeRelations)
			Expect(len(resultIncludeRelations)).To(Equal(3))
			Expect(resultIncludeRelations).To(Equal([]string{"testschema.foo1", "testschema.foo2", "testschema.foo3"}))
		})
	})
	Describe("ConstructColumnPrivilegesMap", func() {
		expectedACL := []ddl.ACL{{Grantee: "gpadmin", Select: true}}
		colI := ddl.ColumnPrivilegesQueryStruct{TableOid: 1, Name: "i", Privileges: sql.NullString{String: "gpadmin=r/gpadmin", Valid: true}, Kind: ""}
		colJ := ddl.ColumnPrivilegesQueryStruct{TableOid: 1, Name: "j", Privileges: sql.NullString{String: "gpadmin=r/gpadmin", Valid: true}, Kind: ""}
		colK1 := ddl.ColumnPrivilegesQueryStruct{TableOid: 2, Name: "k", Privileges: sql.NullString{String: "gpadmin=r/gpadmin", Valid: true}, Kind: ""}
		colK2 := ddl.ColumnPrivilegesQueryStruct{TableOid: 2, Name: "k", Privileges: sql.NullString{String: "testrole=r/testrole", Valid: true}, Kind: ""}
		colDefault := ddl.ColumnPrivilegesQueryStruct{TableOid: 2, Name: "l", Privileges: sql.NullString{String: "", Valid: false}, Kind: "Default"}
		colEmpty := ddl.ColumnPrivilegesQueryStruct{TableOid: 2, Name: "m", Privileges: sql.NullString{String: "", Valid: false}, Kind: "Empty"}
		privileges := []ddl.ColumnPrivilegesQueryStruct{}
		BeforeEach(func() {
			privileges = []ddl.ColumnPrivilegesQueryStruct{}
		})
		It("No columns", func() {
			metadataMap := ddl.ConstructColumnPrivilegesMap(privileges)
			Expect(len(metadataMap)).To(Equal(0))
		})
		It("One column", func() {
			privileges = []ddl.ColumnPrivilegesQueryStruct{colI}
			metadataMap := ddl.ConstructColumnPrivilegesMap(privileges)
			Expect(len(metadataMap)).To(Equal(1))
			Expect(len(metadataMap[1])).To(Equal(1))
			Expect(metadataMap[1]["i"]).To(Equal(expectedACL))
		})
		It("Multiple columns on same table", func() {
			privileges = []ddl.ColumnPrivilegesQueryStruct{colI, colJ}
			metadataMap := ddl.ConstructColumnPrivilegesMap(privileges)
			Expect(len(metadataMap)).To(Equal(1))
			Expect(len(metadataMap[1])).To(Equal(2))
			Expect(metadataMap[1]["i"]).To(Equal(expectedACL))
			Expect(metadataMap[1]["j"]).To(Equal(expectedACL))
		})
		It("Multiple columns on multiple tables", func() {
			privileges = []ddl.ColumnPrivilegesQueryStruct{colI, colJ, colK1, colK2}
			metadataMap := ddl.ConstructColumnPrivilegesMap(privileges)

			expectedACLForK := []ddl.ACL{{Grantee: "gpadmin", Select: true}, {Grantee: "testrole", Select: true}}

			Expect(len(metadataMap)).To(Equal(2))
			Expect(len(metadataMap[1])).To(Equal(2))
			Expect(len(metadataMap[2])).To(Equal(1))
			Expect(metadataMap[1]["i"]).To(Equal(expectedACL))
			Expect(metadataMap[1]["j"]).To(Equal(expectedACL))
			Expect(metadataMap[2]["k"]).To(Equal(expectedACLForK))
		})
		It("Default kind", func() {
			privileges = []ddl.ColumnPrivilegesQueryStruct{colDefault}
			metadataMap := ddl.ConstructColumnPrivilegesMap(privileges)

			expectedACLForDefaultKind := []ddl.ACL{}

			Expect(len(metadataMap)).To(Equal(1))
			Expect(len(metadataMap[2])).To(Equal(1))
			Expect(metadataMap[2]["l"]).To(Equal(expectedACLForDefaultKind))
		})
		It("'Empty' kind", func() {
			privileges = []ddl.ColumnPrivilegesQueryStruct{colEmpty}
			metadataMap := ddl.ConstructColumnPrivilegesMap(privileges)

			expectedACLForEmptyKind := []ddl.ACL{{Grantee: "GRANTEE"}}

			Expect(len(metadataMap)).To(Equal(1))
			Expect(len(metadataMap[2])).To(Equal(1))
			Expect(metadataMap[2]["m"]).To(Equal(expectedACLForEmptyKind))
		})
	})
})