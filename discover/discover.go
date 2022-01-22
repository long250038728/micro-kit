package discover

//Discover 面向接口编程
type Discover interface {
	// Register 服务注册
	Register(serviceName string,
		instanceHost string,
		instancePort int,
		mete map[string]string,
		healthUrl string,
	) bool

	// DeRegister 服务取消
	DeRegister() bool

	// DiscoverServices 获取该服务名对应的列表
	DiscoverServices()[]interface{}
}