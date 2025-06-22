package api

type ServiceInstance struct {
	ID          string `json:"id"`
	ServiceName string `json:"serviceName"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	URL         string `json:"url"`
	HealthPath  string `json:"healthPath"`
}
