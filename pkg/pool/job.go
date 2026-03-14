package pool

// JobFunc 任务执行函数
type JobFunc func(id int64, data interface{})

// Job 任务
type Job struct {
	Data    interface{}
	JobFunc JobFunc
}
