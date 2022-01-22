package discover

import (
	"fmt"
	"github.com/go-kit/kit/sd/consul"
	"github.com/hashicorp/consul/api"
	"github.com/hashicorp/consul/api/watch"
	"github.com/long250038728/micro-kit/discover/dto"
	uuid "github.com/satori/go.uuid"
	"strconv"
	"sync"
)

func NewDiscoverClient(consulUrl string, consulPort int) (Discover, error) {
	consulConfig := api.DefaultConfig()
	consulConfig.Address = consulUrl + ":" + strconv.Itoa(consulPort)

	apiClient,err := api.NewClient(consulConfig)
	if err != nil {
		return nil, err
	}

	client := consul.NewClient(apiClient)
	return &Client{
		Host:   consulUrl,
		Port:   consulPort,
		config: consulConfig,
		client: client,
	}, err
}

// Client
// 1.把服务的host port 进行存放
// 2.初始化的时候生成 consul  consul.config 也放在里面
// 3.instancesMap 用于快速读取consul中service name对应的列表（缓存）
// 4. mutex 用于写instancesMap时加锁
type Client struct {
	Host string `json:"host"`
	Port        int             `json:"port"`
	ServiceInfo dto.ServiceInfo `json:"service_info"`

	client consul.Client    //consul客户端
	config *api.Config      //consul配置

	mutex sync.Mutex		//用于写instancesMap加锁
	instancesMap sync.Map   //用于快速查找对应的服务列表
}

//Register 服务注册
// 传入服务名、端口号等参数进行服务注册 （由于consul在init初始化的时候就已经创建consul对象）
func (discoverClient *Client) Register(serviceName string, instanceHost string, instancePort int, mete map[string]string, healthUrl string)bool{
	instanceId := serviceName + "-" +  uuid.NewV4().String()
	serviceInfo := dto.ServiceInfo{
		ServiceName: serviceName,
		ServiceHost: instanceHost,
		ServicePort: instancePort,
		InstanceId: instanceId,
	}
	discoverClient.ServiceInfo = serviceInfo


	//生成consul中服务实例
	serviceRegistration := &api.AgentServiceRegistration{
		ID: instanceId,
		Name: serviceName,
		Address: instanceHost,
		Port: instancePort,
		Meta: mete,
		Check: &api.AgentServiceCheck{
			DeregisterCriticalServiceAfter: "30s",
			HTTP: "http://" + instanceHost + ":" + strconv.Itoa(instancePort) + healthUrl,
			Interval: "15s",
		},
	}

	//将服务实例注册到consul中
	err := discoverClient.client.Register(serviceRegistration)
	if err != nil {
		fmt.Println("Register service error")
		return  false
	}
	fmt.Println("Register service success")
	return true
}





//DeRegister 解除服务注册（由于consul在init初始化的时候就已经创建consul对象）
func (discoverClient *Client) DeRegister()bool{
	instanceId  := discoverClient.ServiceInfo.InstanceId
	if instanceId == "" {
		return false
	}

	serviceRegistration := &api.AgentServiceRegistration{
		ID: instanceId,
	}
	err := discoverClient.client.Deregister(serviceRegistration)
	if err != nil {
		fmt.Println("DeRegister service error")
		return false
	}
	fmt.Println("DeRegister service success")
	return  true
}





//DiscoverServices 获取服务列表列表
func (discoverClient *Client) DiscoverServices()[]interface{} {
	serviceName := discoverClient.ServiceInfo.ServiceName
	if serviceName == "" {
		return []interface{}{}
	}

	//对象中存在就返回对象中的数据
	if instanceList , ok := discoverClient.instancesMap.Load(serviceName) ; ok {
		return instanceList.([]interface{})
	}

	//加锁
	discoverClient.mutex.Lock()
	defer discoverClient.mutex.Unlock()

	//在加锁后再查下看会不会这个时候数据就来了
	if instanceList , ok := discoverClient.instancesMap.Load(serviceName) ; ok {
		return instanceList.([]interface{})
	}

	//注册监控
	go func() {
		//使用 consul 服务实例监控来监控某个服务名的服务实例列表变化
		params := make(map[string]interface{})
		params["type"] = "service"
		params["service"] = serviceName
		plan, _ := watch.Parse(params)
		plan.Handler = func(u uint64, list interface{}) {
			fmt.Println("plan.Handler")

			if list == nil {
				return
			}
			objects, ok := list.([]*api.ServiceEntry)
			if !ok {
				return // 数据异常，忽略
			}


			// 没有服务实例在线
			if len(objects) == 0 {
				discoverClient.instancesMap.Store(serviceName, []interface{}{})
			}

			//遍历
			var healthServices []interface{}
			for _, service := range objects {
				if service.Checks.AggregatedStatus() == api.HealthPassing {
					healthServices = append(healthServices, service.Service)
				}
			}
			discoverClient.instancesMap.Store(serviceName, healthServices)
		}
		defer plan.Stop()
		err := plan.Run(discoverClient.config.Address)
		if err != nil {
			return
		}
	}()

	// 根据服务名获取实例列表
	entries, _, err := discoverClient.client.Service(serviceName, "", false, nil)
	//1.如果获取失败就话就变成空数组
	if err != nil {
		discoverClient.instancesMap.Store(serviceName, []interface{}{})
		fmt.Println("Discover Service Error!")
		return nil
	}
	//2.如果成功就把entries列表中找出service拼接数组
	instances := make([]interface{}, len(entries))
	for i := 0; i < len(instances); i++ {
		instances[i] = entries[i].Service
	}
	discoverClient.instancesMap.Store(serviceName, instances)
	return instances
}