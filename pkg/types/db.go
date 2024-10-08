package types

type DbType string

func (t DbType) String() string {
	return string(t)
}

const (
	BlocksDB          DbType = "blocks"
	IndexDB           DbType = "index"
	MetadataDB        DbType = "metadata"
	TransactionsDB    DbType = "transactions"
	DAGPredecessorsDB DbType = "dag_predecessors"
	DAGSuccessorsDB   DbType = "dag_successors"
)
