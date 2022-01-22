package dto

type ServiceInfo struct {
	ServiceHost string `json:"service_host"`
	ServiceName string `json:"service_name"`
	ServicePort int `json:"service_port"`
	InstanceId string `json:"instance_id"`
}