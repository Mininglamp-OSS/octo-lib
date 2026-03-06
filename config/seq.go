package config

import (
	"fmt"
	"sync"

	"github.com/gocraft/dbr/v2"
)

var seqMap = map[string]*Seq{}
var seqLock sync.Mutex
var seqStep int64 = 1000 // 序列号步长

// Seq Seq
type Seq struct {
	CurSeq int64
	MaxSeq int64
}

// GenSeq generates a monotonically increasing sequence number for the given flag.
// It is safe for concurrent use.
func (c *Context) GenSeq(flag string) (int64, error) {
	seqLock.Lock()
	defer seqLock.Unlock()

	key := fmt.Sprintf("seq:%s", flag)
	seq := seqMap[flag]

	if seq == nil {
		seqM, err := querySeqWithKey(c.DB(), key)
		if err != nil {
			return 0, fmt.Errorf("query seq for %q: %w", flag, err)
		}
		if seqM == nil {
			var currSeq int64 = 1000000
			err = addOrUpdateSeq(c.DB(), &seqModel{
				Key:    key,
				Step:   int(seqStep),
				MinSeq: currSeq + seqStep,
			})
			if err != nil {
				return 0, fmt.Errorf("init seq for %q: %w", flag, err)
			}
			seq = &Seq{
				CurSeq: currSeq,
				MaxSeq: currSeq + seqStep,
			}
		} else {
			seq = &Seq{
				CurSeq: seqM.MinSeq,
				MaxSeq: seqM.MinSeq,
			}
		}
		seqMap[flag] = seq
	}

	if seq.CurSeq >= seq.MaxSeq {
		err := addOrUpdateSeq(c.DB(), &seqModel{
			Key:    key,
			Step:   int(seqStep),
			MinSeq: seq.CurSeq + seqStep,
		})
		if err != nil {
			return 0, fmt.Errorf("extend seq for %q: %w", flag, err)
		}
		seq.MaxSeq += seqStep
	}

	seq.CurSeq++
	return seq.CurSeq, nil
}

func addOrUpdateSeq(session *dbr.Session, m *seqModel) error {
	_, err := session.InsertBySql("insert into `seq`(`key`,min_seq,step) values(?,?,?) ON DUPLICATE KEY UPDATE min_seq=VALUES(min_seq)", m.Key, m.MinSeq, m.Step).Exec()
	return err
}

func querySeqWithKey(session *dbr.Session, key string) (*seqModel, error) {
	var m *seqModel
	_, err := session.Select("*").From("seq").Where("`key`=?", key).Load(&m)
	return m, err
}

type seqModel struct {
	Key    string
	MinSeq int64
	Step   int
}
