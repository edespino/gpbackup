package ddl

import (
	"github.com/greenplum-db/gp-common-go-libs/gplog"
	"github.com/greenplum-db/gpbackup/utils"
)

/*
 * This file contains wrapper functions that group together functions relating
 * to querying and printing metadata, so that the logic for each object type
 * can all be in one place and backup.go can serve as a high-level look at the
 * overall backup flow.
 */

/*
 * Metadata retrieval wrapper functions
 */

func RetrieveAndProcessTables() ([]Relation, []Relation, map[uint32]TableDefinition) {
	gplog.Info("Gathering list of tables for backup")
	tables := GetAllUserTables(connectionPool, leafPartitionData)
	LockTables(connectionPool, tables)

	/*
	 * We expand the includeRelations list to include parent and leaf partitions that may not have been
	 * specified by the user but are used in the backup for metadata or data.
	 */
	userPassedIncludeRelations := includeRelations
	includeRelations = ExpandIncludeRelations(includeRelations, tables)

	tableDefs := ConstructDefinitionsForTables(connectionPool, tables)
	metadataTables, dataTables := SplitTablesByPartitionType(tables, tableDefs, userPassedIncludeRelations)
	ObjectCounts["Tables"] = len(metadataTables)

	return metadataTables, dataTables, tableDefs
}

func RetrieveFunctions(procLangs []ProceduralLanguage) ([]Function, []Function, MetadataMap) {
	gplog.Verbose("Retrieving function information")
	functions := GetFunctionsAllVersions(connectionPool)
	ObjectCounts["Functions"] = len(functions)
	functionMetadata := GetMetadataForObjectType(connectionPool, TYPE_FUNCTION)
	functions = ConstructFunctionDependencies(connectionPool, functions)
	langFuncs, otherFuncs := ExtractLanguageFunctions(functions, procLangs)
	return langFuncs, otherFuncs, functionMetadata
}

func RetrieveTypes() ([]Type, MetadataMap, map[uint32]FunctionInfo) {
	gplog.Verbose("Retrieving type information")
	shells := GetShellTypes(connectionPool)
	bases := GetBaseTypes(connectionPool)
	funcInfoMap := GetFunctionOidToInfoMap(connectionPool)
	if connectionPool.Version.Before("5") {
		bases = ConstructBaseTypeDependencies4(connectionPool, bases, funcInfoMap)
	} else {
		bases = ConstructBaseTypeDependencies5(connectionPool, bases)
	}
	types := append(shells, bases...)
	composites := GetCompositeTypes(connectionPool)
	composites = ConstructCompositeTypeDependencies(connectionPool, composites)
	types = append(types, composites...)
	domains := GetDomainTypes(connectionPool)
	domains = ConstructDomainDependencies(connectionPool, domains)
	types = append(types, domains...)
	ObjectCounts["Types"] = len(types)
	typeMetadata := GetMetadataForObjectType(connectionPool, TYPE_TYPE)
	return types, typeMetadata, funcInfoMap
}

func RetrieveConstraints(tables ...Relation) ([]Constraint, MetadataMap) {
	constraints := GetConstraints(connectionPool, tables...)
	conMetadata := GetCommentsForObjectType(connectionPool, TYPE_CONSTRAINT)
	return constraints, conMetadata
}

func RetrieveSequences() ([]Sequence, map[string]string) {
	sequenceOwnerTables, sequenceOwnerColumns := GetSequenceColumnOwnerMap(connectionPool)
	sequences := GetAllSequences(connectionPool, sequenceOwnerTables)
	return sequences, sequenceOwnerColumns
}

/*
 * Generic metadata wrapper functions
 */

func BackupSessionGUCs(metadataFile *utils.FileWithByteCount) {
	gucs := GetSessionGUCs(connectionPool)
	PrintSessionGUCs(metadataFile, globalTOC, gucs)
}

/*
 * Global metadata wrapper functions
 */

func BackupTablespaces(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE TABLESPACE statements to metadata file")
	tablespaces := GetTablespaces(connectionPool)
	ObjectCounts["Tablespaces"] = len(tablespaces)
	tablespaceMetadata := GetMetadataForObjectType(connectionPool, TYPE_TABLESPACE)
	PrintCreateTablespaceStatements(metadataFile, globalTOC, tablespaces, tablespaceMetadata)
}

func BackupCreateDatabase(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE DATABASE statement to metadata file")
	db := GetDatabaseInfo(connectionPool)
	dbMetadata := GetMetadataForObjectType(connectionPool, TYPE_DATABASE)
	PrintCreateDatabaseStatement(metadataFile, globalTOC, db, dbMetadata)
}

func BackupDatabaseGUCs(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing database GUCs to metadata file")
	databaseGucs := GetDatabaseGUCs(connectionPool)
	ObjectCounts["Database GUCs"] = len(databaseGucs)
	PrintDatabaseGUCs(metadataFile, globalTOC, databaseGucs, connectionPool.DBName)
}

func BackupResourceQueues(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE RESOURCE QUEUE statements to metadata file")
	resQueues := GetResourceQueues(connectionPool)
	ObjectCounts["Resource Queues"] = len(resQueues)
	resQueueMetadata := GetCommentsForObjectType(connectionPool, TYPE_RESOURCEQUEUE)
	PrintCreateResourceQueueStatements(metadataFile, globalTOC, resQueues, resQueueMetadata)
}

func BackupResourceGroups(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE RESOURCE GROUP statements to metadata file")
	resGroups := GetResourceGroups(connectionPool)
	ObjectCounts["Resource Groups"] = len(resGroups)
	resGroupMetadata := GetCommentsForObjectType(connectionPool, TYPE_RESOURCEGROUP)
	PrintCreateResourceGroupStatements(metadataFile, globalTOC, resGroups, resGroupMetadata)
}

func BackupRoles(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE ROLE statements to metadata file")
	roles := GetRoles(connectionPool)
	ObjectCounts["Roles"] = len(roles)
	roleGUCs := GetRoleGUCs(connectionPool)
	roleMetadata := GetCommentsForObjectType(connectionPool, TYPE_ROLE)
	PrintCreateRoleStatements(metadataFile, globalTOC, roles, roleGUCs, roleMetadata)
}

func BackupRoleGrants(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing GRANT ROLE statements to metadata file")
	roleMembers := GetRoleMembers(connectionPool)
	PrintRoleMembershipStatements(metadataFile, globalTOC, roleMembers)
}

/*
 * Predata wrapper functions
 */

func BackupSchemas(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE SCHEMA statements to metadata file")
	schemas := GetAllUserSchemas(connectionPool)
	ObjectCounts["Schemas"] = len(schemas)
	schemaMetadata := GetMetadataForObjectType(connectionPool, TYPE_SCHEMA)
	PrintCreateSchemaStatements(metadataFile, globalTOC, schemas, schemaMetadata)
}

func BackupProceduralLanguages(metadataFile *utils.FileWithByteCount, procLangs []ProceduralLanguage, langFuncs []Function, functionMetadata MetadataMap, funcInfoMap map[uint32]FunctionInfo) {
	gplog.Verbose("Writing CREATE PROCEDURAL LANGUAGE statements to metadata file")
	ObjectCounts["Procedural Languages"] = len(procLangs)
	for _, langFunc := range langFuncs {
		PrintCreateFunctionStatement(metadataFile, globalTOC, langFunc, functionMetadata[langFunc.Oid])
	}
	procLangMetadata := GetMetadataForObjectType(connectionPool, TYPE_PROCLANGUAGE)
	PrintCreateLanguageStatements(metadataFile, globalTOC, procLangs, funcInfoMap, procLangMetadata)
}

func BackupForeignDataWrappers(metadataFile *utils.FileWithByteCount, funcInfoMap map[uint32]FunctionInfo) {
	gplog.Verbose("Writing CREATE FOREIGN DATA WRAPPER statements to metadata file")
	wrappers := GetForeignDataWrappers(connectionPool)
	ObjectCounts["Foreign Data Wrappers"] = len(wrappers)
	fdwMetadata := GetMetadataForObjectType(connectionPool, TYPE_FOREIGNDATAWRAPPER)
	PrintCreateForeignDataWrapperStatements(metadataFile, globalTOC, wrappers, funcInfoMap, fdwMetadata)
}

func BackupForeignServers(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE SERVER statements to metadata file")
	servers := GetForeignServers(connectionPool)
	ObjectCounts["Foreign Servers"] = len(servers)
	serverMetadata := GetMetadataForObjectType(connectionPool, TYPE_FOREIGNSERVER)
	PrintCreateServerStatements(metadataFile, globalTOC, servers, serverMetadata)
}

func BackupUserMappings(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE USER MAPPING statements to metadata file")
	mappings := GetUserMappings(connectionPool)
	ObjectCounts["User Mappings"] = len(mappings)
	PrintCreateUserMappingStatements(metadataFile, globalTOC, mappings)
}

func BackupShellTypes(metadataFile *utils.FileWithByteCount, types []Type) {
	gplog.Verbose("Writing CREATE TYPE statements for shell types to metadata file")
	PrintCreateShellTypeStatements(metadataFile, globalTOC, types)
}

func BackupEnumTypes(metadataFile *utils.FileWithByteCount, typeMetadata MetadataMap) {
	enums := GetEnumTypes(connectionPool)
	gplog.Verbose("Writing CREATE TYPE statements for enum types to metadata file")
	ObjectCounts["Types"] += len(enums)
	PrintCreateEnumTypeStatements(metadataFile, globalTOC, enums, typeMetadata)
}

func BackupCreateSequences(metadataFile *utils.FileWithByteCount, sequences []Sequence, relationMetadata MetadataMap) {
	gplog.Verbose("Writing CREATE SEQUENCE statements to metadata file")
	ObjectCounts["Sequences"] = len(sequences)
	PrintCreateSequenceStatements(metadataFile, globalTOC, sequences, relationMetadata)
}

// This function is fairly unwieldy, but there's not really a good way to break it down
func BackupFunctionsAndTypesAndTables(metadataFile *utils.FileWithByteCount, otherFuncs []Function, types []Type, tables []Relation, functionMetadata MetadataMap, typeMetadata MetadataMap, relationMetadata MetadataMap, tableDefs map[uint32]TableDefinition, constraints []Constraint) {
	gplog.Verbose("Writing CREATE FUNCTION statements to metadata file")
	gplog.Verbose("Writing CREATE TYPE statements for base, composite, and domain types to metadata file")
	gplog.Verbose("Writing CREATE TABLE statements to metadata file")
	tables = ConstructTableDependencies(connectionPool, tables, tableDefs, false)
	sortedSlice := SortFunctionsAndTypesAndTablesInDependencyOrder(otherFuncs, types, tables)
	filteredMetadata := ConstructFunctionAndTypeAndTableMetadataMap(functionMetadata, typeMetadata, relationMetadata)
	PrintCreateDependentTypeAndFunctionAndTablesStatements(metadataFile, globalTOC, sortedSlice, filteredMetadata, tableDefs, constraints)
	extPartInfo, partInfoMap := GetExternalPartitionInfo(connectionPool)
	if len(extPartInfo) > 0 {
		gplog.Verbose("Writing EXCHANGE PARTITION statements to metadata file")
		PrintExchangeExternalPartitionStatements(metadataFile, globalTOC, extPartInfo, partInfoMap, tables)
	}
}

// This function should be used only with a table-only backup.  For an unfiltered backup, the above function is used.
func BackupTables(metadataFile *utils.FileWithByteCount, tables []Relation, relationMetadata MetadataMap, tableDefs map[uint32]TableDefinition, constraints []Constraint) {
	gplog.Verbose("Writing CREATE TABLE statements to metadata file")
	tables = ConstructTableDependencies(connectionPool, tables, tableDefs, true)
	sortable := make([]Sortable, 0)
	for _, table := range tables {
		sortable = append(sortable, table)
	}
	sortedSlice := TopologicalSort(sortable)
	PrintCreateDependentTypeAndFunctionAndTablesStatements(metadataFile, globalTOC, sortedSlice, relationMetadata, tableDefs, constraints)
	extPartInfo, partInfoMap := GetExternalPartitionInfo(connectionPool)
	if len(extPartInfo) > 0 {
		gplog.Verbose("Writing EXCHANGE PARTITION statements to metadata file")
		PrintExchangeExternalPartitionStatements(metadataFile, globalTOC, extPartInfo, partInfoMap, tables)
	}
}

func BackupProtocols(metadataFile *utils.FileWithByteCount, funcInfoMap map[uint32]FunctionInfo) {
	gplog.Verbose("Writing CREATE PROTOCOL statements to metadata file")
	protocols := GetExternalProtocols(connectionPool)
	ObjectCounts["Protocols"] = len(protocols)
	protoMetadata := GetMetadataForObjectType(connectionPool, TYPE_PROTOCOL)
	PrintCreateExternalProtocolStatements(metadataFile, globalTOC, protocols, funcInfoMap, protoMetadata)
}

func BackupTSParsers(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE TEXT SEARCH PARSER statements to metadata file")
	parsers := GetTextSearchParsers(connectionPool)
	ObjectCounts["Text Search Parsers"] = len(parsers)
	parserMetadata := GetCommentsForObjectType(connectionPool, TYPE_TSPARSER)
	PrintCreateTextSearchParserStatements(metadataFile, globalTOC, parsers, parserMetadata)
}

func BackupTSTemplates(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE TEXT SEARCH TEMPLATE statements to metadata file")
	templates := GetTextSearchTemplates(connectionPool)
	ObjectCounts["Text Search Templates"] = len(templates)
	templateMetadata := GetCommentsForObjectType(connectionPool, TYPE_TSTEMPLATE)
	PrintCreateTextSearchTemplateStatements(metadataFile, globalTOC, templates, templateMetadata)
}

func BackupTSDictionaries(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE TEXT SEARCH DICTIONARY statements to metadata file")
	dictionaries := GetTextSearchDictionaries(connectionPool)
	ObjectCounts["Text Search Dictionaries"] = len(dictionaries)
	dictionaryMetadata := GetMetadataForObjectType(connectionPool, TYPE_TSDICTIONARY)
	PrintCreateTextSearchDictionaryStatements(metadataFile, globalTOC, dictionaries, dictionaryMetadata)
}

func BackupTSConfigurations(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE TEXT SEARCH CONFIGURATION statements to metadata file")
	configurations := GetTextSearchConfigurations(connectionPool)
	ObjectCounts["Text Search Configurations"] = len(configurations)
	configurationMetadata := GetMetadataForObjectType(connectionPool, TYPE_TSCONFIGURATION)
	PrintCreateTextSearchConfigurationStatements(metadataFile, globalTOC, configurations, configurationMetadata)
}

func BackupConversions(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE CONVERSION statements to metadata file")
	conversions := GetConversions(connectionPool)
	ObjectCounts["Conversions"] = len(conversions)
	convMetadata := GetMetadataForObjectType(connectionPool, TYPE_CONVERSION)
	PrintCreateConversionStatements(metadataFile, globalTOC, conversions, convMetadata)
}

func BackupOperators(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE OPERATOR statements to metadata file")
	operators := GetOperators(connectionPool)
	ObjectCounts["Operators"] = len(operators)
	operatorMetadata := GetMetadataForObjectType(connectionPool, TYPE_OPERATOR)
	PrintCreateOperatorStatements(metadataFile, globalTOC, operators, operatorMetadata)
}

func BackupOperatorFamilies(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE OPERATOR FAMILY statements to metadata file")
	operatorFamilies := GetOperatorFamilies(connectionPool)
	ObjectCounts["Operator Families"] = len(operatorFamilies)
	operatorFamilyMetadata := GetMetadataForObjectType(connectionPool, TYPE_OPERATORFAMILY)
	PrintCreateOperatorFamilyStatements(metadataFile, globalTOC, operatorFamilies, operatorFamilyMetadata)
}

func BackupOperatorClasses(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE OPERATOR CLASS statements to metadata file")
	operatorClasses := GetOperatorClasses(connectionPool)
	ObjectCounts["Operator Classes"] = len(operatorClasses)
	operatorClassMetadata := GetMetadataForObjectType(connectionPool, TYPE_OPERATORCLASS)
	PrintCreateOperatorClassStatements(metadataFile, globalTOC, operatorClasses, operatorClassMetadata)
}

func BackupAggregates(metadataFile *utils.FileWithByteCount, funcInfoMap map[uint32]FunctionInfo) {
	gplog.Verbose("Writing CREATE AGGREGATE statements to metadata file")
	aggregates := GetAggregates(connectionPool)
	ObjectCounts["Aggregates"] = len(aggregates)
	aggMetadata := GetMetadataForObjectType(connectionPool, TYPE_AGGREGATE)
	PrintCreateAggregateStatements(metadataFile, globalTOC, aggregates, funcInfoMap, aggMetadata)
}

func BackupCasts(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE CAST statements to metadata file")
	casts := GetCasts(connectionPool)
	ObjectCounts["Casts"] = len(casts)
	castMetadata := GetCommentsForObjectType(connectionPool, TYPE_CAST)
	PrintCreateCastStatements(metadataFile, globalTOC, casts, castMetadata)
}

func BackupCollations(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE COLLATION statements to metadata file")
	collations := GetCollations(connectionPool)
	ObjectCounts["Collations"] = len(collations)
	collationMetadata := GetMetadataForObjectType(connectionPool, TYPE_COLLATION)
	PrintCreateCollationStatements(metadataFile, globalTOC, collations, collationMetadata)
}

func BackupExtensions(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE EXTENSION statements to metadata file")
	extensions := GetExtensions(connectionPool)
	ObjectCounts["Extensions"] = len(extensions)
	extensionMetadata := GetCommentsForObjectType(connectionPool, TYPE_EXTENSION)
	PrintCreateExtensionStatements(metadataFile, globalTOC, extensions, extensionMetadata)
}

func BackupViews(metadataFile *utils.FileWithByteCount, relationMetadata MetadataMap) {
	gplog.Verbose("Writing CREATE VIEW statements to metadata file")
	views := GetViews(connectionPool)
	ObjectCounts["Views"] = len(views)
	views = ConstructViewDependencies(connectionPool, views)
	views = SortViews(views)
	PrintCreateViewStatements(metadataFile, globalTOC, views, relationMetadata)
}

func BackupConstraints(metadataFile *utils.FileWithByteCount, constraints []Constraint, conMetadata MetadataMap) {
	gplog.Verbose("Writing ADD CONSTRAINT statements to metadata file")
	PrintConstraintStatements(metadataFile, globalTOC, constraints, conMetadata)
}

/*
 * Postdata wrapper functions
 */

func BackupIndexes(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE INDEX statements to metadata file")
	indexes := GetIndexes(connectionPool)
	ObjectCounts["Indexes"] = len(indexes)
	indexMetadata := GetCommentsForObjectType(connectionPool, TYPE_INDEX)
	PrintCreateIndexStatements(metadataFile, globalTOC, indexes, indexMetadata)
}

func BackupRules(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE RULE statements to metadata file")
	rules := GetRules(connectionPool)
	ObjectCounts["Rules"] = len(rules)
	ruleMetadata := GetCommentsForObjectType(connectionPool, TYPE_RULE)
	PrintCreateRuleStatements(metadataFile, globalTOC, rules, ruleMetadata)
}

func BackupTriggers(metadataFile *utils.FileWithByteCount) {
	gplog.Verbose("Writing CREATE TRIGGER statements to metadata file")
	triggers := GetTriggers(connectionPool)
	ObjectCounts["Triggers"] = len(triggers)
	triggerMetadata := GetCommentsForObjectType(connectionPool, TYPE_TRIGGER)
	PrintCreateTriggerStatements(metadataFile, globalTOC, triggers, triggerMetadata)
}
