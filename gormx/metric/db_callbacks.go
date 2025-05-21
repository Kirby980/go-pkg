package metric

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
)

type Callbacks struct {
	vector *prometheus.SummaryVec
}

func NewCallbacks(namespace, subsystem, name, help string, objectives map[float64]float64, opt ...string) *Callbacks {
	vector := prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Namespace:  namespace,
		Subsystem:  subsystem,
		Name:       name,
		Help:       help,
		Objectives: objectives,
	}, opt)
	pcb := &Callbacks{
		vector: vector,
	}
	prometheus.MustRegister(vector)
	return pcb

}

func (c *Callbacks) Name() string {
	return "prometheus-query-time"
}
func (c *Callbacks) Initialize(db *gorm.DB) error {
	c.RegisterAll(db)
	return nil
}

func (c *Callbacks) Before() func(db *gorm.DB) {
	return func(db *gorm.DB) {
		start := time.Now()
		db.Set("start", start)
	}
}
func (c *Callbacks) After(typ string) func(db *gorm.DB) {
	return func(db *gorm.DB) {
		start, _ := db.Get("start")
		startTime, ok := start.(time.Time)
		if !ok {
			return
		}
		table := db.Statement.Table
		if table == "" {
			table = "unknown"
		}
		c.vector.WithLabelValues(typ, table).Observe(float64(time.Since(startTime).Milliseconds()))
	}
}

func (c *Callbacks) RegisterAll(db *gorm.DB) {
	err := db.Callback().Create().Before("*").Register("prometheus_create_before", c.Before())
	if err != nil {
		panic(err)
	}
	err = db.Callback().Create().After("*").Register("prometheus_create_after", c.After("create"))
	if err != nil {
		panic(err)
	}
	err = db.Callback().Update().Before("*").Register("prometheus_update_before", c.Before())
	if err != nil {
		panic(err)
	}
	err = db.Callback().Update().After("*").Register("prometheus_update_after", c.After("update"))
	if err != nil {
		panic(err)
	}
	err = db.Callback().Delete().Before("*").Register("prometheus_delete_before", c.Before())
	if err != nil {
		panic(err)
	}
	err = db.Callback().Delete().After("*").Register("prometheus_delete_after", c.After("delete"))
	if err != nil {
		panic(err)
	}
	err = db.Callback().Query().Before("*").Register("prometheus_query_before", c.Before())
	if err != nil {
		panic(err)
	}
	err = db.Callback().Query().After("*").Register("prometheus_query_after", c.After("query"))
	if err != nil {
		panic(err)
	}
	err = db.Callback().Raw().Before("*").Register("prometheus_raw_before", c.Before())
	if err != nil {
		panic(err)
	}
	err = db.Callback().Raw().After("*").Register("prometheus_raw_after", c.After("raw"))
	if err != nil {
		panic(err)
	}
	err = db.Callback().Row().Before("*").Register("prometheus_row_before", c.Before())
	if err != nil {
		panic(err)
	}
	err = db.Callback().Row().After("*").Register("prometheus_row_after", c.After("row"))
	if err != nil {
		panic(err)
	}
}
