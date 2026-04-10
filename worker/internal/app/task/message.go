package task

type VideoTaskMessage struct {
	TaskID    string                 `json:"task_id"`
	TaskType  string                 `json:"task_type"`
	Payload   map[string]interface{} `json:"payload"`
	CreatedAt string                 `json:"created_at"`
}
