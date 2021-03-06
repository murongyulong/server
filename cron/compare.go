package cron

import (
	"fmt"
	"log"
	"strings"
	"time"
	"math/rand"
	"os"
	"net"
	
  	"golang.org/x/crypto/ssh" 
	"github.com/murongyulong/common/model"
	"github.com/murongyulong/server/g"
	"github.com/murongyulong/go-dockerclient"
	"github.com/toolkits/slice"
)

func getDesiredState() (map[string]*model.App, error) {
	sql := "select a.id id, a.app_name name, a.app_memory memory, a.app_instance instance, a.app_image image, a.app_status status, a.app_port port, a.app_mount mount, a.app_cpushares cpushares from ysy_app a where a.app_status = 0 and a.app_image <> ''"
	rows, err := g.DB.Query(sql)
	if err != nil { 
		log.Printf("[ERROR] exec %s fail: %s", sql, err)
		return nil, err
	}

	var desiredState = make(map[string]*model.App)
	for rows.Next() {
		var app model.App
		err = rows.Scan(&app.Id,&app.Name, &app.Memory, &app.InstanceCnt, &app.Image, &app.Status, &app.Port, &app.Mount, &app.CPUShares)
		if err != nil {
			log.Printf("[ERROR] %s scan fail: %s", sql, err)
			return nil, err
		}

		desiredState[app.Name] = &app
	}

	return desiredState, nil
}

func CompareState() {
	duration := time.Duration(g.Config().Interval) * time.Second
	time.Sleep(duration)
	for {
		time.Sleep(duration)
		compareState()
	}
}

func compareState() {
	desiredState, err := getDesiredState()
	if err != nil {
		log.Println("[ERROR] get desired state fail:", err)
		return
	}

	debug := g.Config().Debug

	if debug {
		log.Println("comparing......")
	}

	if len(desiredState) == 0 {
		if debug {
			log.Println("no desired app. do nothing")
		}
		// do nothing.
		return
	}

	newAppSlice := []string{}

	for name, app := range desiredState {
		if !g.RealState.RealAppExists(name) {
			if debug && app.InstanceCnt > 0 {
				log.Println("[=-NEW-=]:", name)
			}
			newAppSlice = append(newAppSlice, name)
			createNewContainer(app, app.InstanceCnt)
		}
	}

	realNames := g.RealState.Keys()

	for ii, name := range realNames {
		if debug {
			log.Printf("#%d: %s", ii, name)
		}

		if slice.ContainsString(newAppSlice, name) {
			continue
		}

		app, exists := desiredState[name]
		if !exists {
			if debug {
				log.Println("[=-DEL-=]:", name)
			}
			dropApp(name)
			continue
		}

		sa, _ := g.RealState.GetSafeApp(name)
		isOld, olds := sa.IsOldVersion(app.Image)
		if isOld {
			if len(olds) > 0 || app.InstanceCnt > 0 {
				log.Println("[=-UPGRADE-=]")
			}
			// deploy new instances
			createNewContainer(app, app.InstanceCnt)
			// delete old instances
			for _, c := range olds {
				dropContainer(c)
			}

			continue
		}

		nowCnt := sa.ContainerCount()

		if nowCnt < app.InstanceCnt {
			if debug {
				log.Printf("add:%d", app.InstanceCnt-nowCnt)
			}
			createNewContainer(app, app.InstanceCnt-nowCnt)
			continue
		}

		if nowCnt > app.InstanceCnt {
			if debug {
				log.Printf("del:%d", nowCnt-app.InstanceCnt)
			}
			dropContainers(sa.Containers(), nowCnt-app.InstanceCnt)
		}
				
	}
}

//建立新的容器
func createNewContainer(app *model.App, deployCnt int) {
	if deployCnt == 0 {
		return
	}

	if app.Status != model.AppStatus_Success {
		if g.Config().Debug {
			log.Printf("!!! App=%s Status = %d", app.Name, app.Status)
		}
		return
	}

	ip_count := g.ChooseNode(app, deployCnt)
	if len(ip_count) == 0 {
		log.Println("no node..zZ")
		return
	}

	for ip, count := range ip_count {
		for k := 0; k < count; k++ {
			DockerRun(app, ip)
		}
	}
}

func dropApp(appName string) {
	if appName == "" {
		return
	}

	if g.Config().Debug {
		log.Println("drop app:", appName)
	}

	sa, _ := g.RealState.GetSafeApp(appName)
	cs := sa.Containers()
	for _, c := range cs {
		dropContainer(c)
	}
	g.RealState.DeleteSafeApp(appName)

	rc := g.RedisConnPool.Get()
	defer rc.Close()

	uriKey := fmt.Sprintf("%s%s.%s", g.Config().Redis.RsPrefix, appName, g.Config().Domain)
	rc.Do("DEL", uriKey)
}

func dropContainers(cs []*model.Container, cnt int) {
	if cnt == 0 {
		return
	}
	done := 0
	for _, c := range cs {
	
		dropContainer(c)
			done++
	
		if done == cnt {
			break
		}
	}
}

func dropContainer(c *model.Container) {

	if g.Config().Debug {
		log.Println("drop container:", c)
	}
	/*----------------------------edit by lianzhi20180706------------------------------------*/
	/*addr := fmt.Sprintf("http://%s:%d", c.Ip, g.Config().DockerPort)
	client, err := docker.NewClient(addr)
	if err != nil {
		log.Println("docker.NewClient fail:", err)
		return
	}*/
	addr := fmt.Sprintf("http://%s:%d", c.Ip, g.Config().DockerPort)
	/*cert_temp, err := ioutil.ReadFile("/go/src/github.com/dinp/cert.pem")
	cert := string(cert_temp)
	if err != nil {
	     fmt.Print(err)
	}
	fmt.Println(cert)
	key_temp, err := ioutil.ReadFile("/go/src/github.com/dinp/key.pem")
	key := string(key_temp)
	if err != nil {
	     fmt.Print(err)
	}
	fmt.Println(key)
	ca_temp, err := ioutil.ReadFile("/go/src/github.com/dinp/ca.pem")
	ca := string(ca_temp)
	if err != nil {
	     fmt.Print(err)
	}
	fmt.Println(ca)*/
	client, err := docker.NewTLSClient(addr,"/go/src/github.com/dinp/cert.pem","/go/src/github.com/dinp/key.pem","/go/src/github.com/dinp/ca.pem")
	if err != nil {
		log.Println("docker.NewClient fail:", err)
		return
	}
	/*----------------------------edit by lianzhi20180706------------------------------------*/
		var id string
	rows := g.DB.QueryRow("select app_id  from  ysy_app_container where con_id =?", c.Id);
    	rows.Scan(&id)
    fmt.Println(id)
	var num string
	rows1 := g.DB.QueryRow("select app_status from ysy_app_status where app_id = ?", id);
    	rows1.Scan(&num)
    fmt.Println(num)
		if num=="0"{

	err = client.RemoveContainer(docker.RemoveContainerOptions{ID: c.Id, Force: true})
	if err != nil {
		log.Println("docker.RemoveContainer fail:", err)
		return
	}
stmt, err := g.DB.Prepare("delete  from  ysy_app_container where con_id =?")
	if err != nil {
   	 log.Println(err)
	}
	log.Println("c.Id", c.Id)
	res,err:= stmt.Exec(c.Id)
	if err != nil {
   	 log.Println(err)
	}
	 affect, err := res.RowsAffected()  
		if err != nil {
   	 log.Println(err)
	}
    fmt.Println(affect)

	// remember to delete real state map item
	sa, exists := g.RealState.GetSafeApp(c.AppName)
	if exists {
		sa.DeleteContainer(c)
	}
	}else{
		log.Println("启动容器不能删除", c.Id)		
		}
}

func BuildEnvArray(envVars map[string]string) []string {
	size := len(envVars)
	if size == 0 {
		return []string{}
	}

	arr := make([]string, size)
	idx := 0
	for k, v := range envVars {
		arr[idx] = fmt.Sprintf("%s=%s", k, v)
		idx++
	}

	return arr
}

func ParseRepositoryTag(repos string) (string, string) {
	n := strings.LastIndex(repos, ":")
	if n < 0 {
		return repos, ""
	}
	if tag := repos[n+1:]; !strings.Contains(tag, "/") {
		return repos[:n], tag
	}
	return repos, ""
}
func connect(user, password, host string, port int) (*ssh.Session, error) { 
  var (
    auth         []ssh.AuthMethod
    addr         string
    clientConfig *ssh.ClientConfig
    client       *ssh.Client
    session      *ssh.Session
    err          error
  )
  // get auth method
  auth = make([]ssh.AuthMethod, 0)
  auth = append(auth, ssh.Password(password))
 
  clientConfig = &ssh.ClientConfig{
    User:    user,
    Auth:    auth,
    Timeout: 30 * time.Second,
    HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
         return nil
}, 
  }
 
  // connet to ssh
  addr = fmt.Sprintf("%s:%d", host, port)
 
  if client, err = ssh.Dial("tcp", addr, clientConfig); err != nil {
    return nil, err
  }
 
  // create session
  if session, err = client.NewSession(); err != nil {
    return nil, err
  }
 
  return session, nil
}
func DockerRun(app *model.App, ip string) {
	if g.Config().Debug {
		log.Printf("create container. app:%s, ip:%s\n", app.Name, ip)
	}

	envVars, err := g.LoadEnvVarsOf(app.Name)
	if err != nil {
		log.Println("[ERROR] load env fail:", err)
		return
	}

	envVars["APP_NAME"] = app.Name
	envVars["HOST_IP"] = ip
	if g.Config().Scribe.Ip != "" {
		envVars["SCRIBE_IP"] = g.Config().Scribe.Ip
	} else {
		envVars["SCRIBE_IP"] = ip
	}
	envVars["SCRIBE_PORT"] = fmt.Sprintf("%d", g.Config().Scribe.Port)

	
	/*----------------------------edit by lianzhi20180706------------------------------------*/
	/*addr := fmt.Sprintf("http://%s:%d", ip, g.Config().DockerPort)

	client, err := docker.NewClient(addr)*/
	addr := fmt.Sprintf("http://%s:%d", ip, g.Config().DockerPort)
	/*cert_temp, err := ioutil.ReadFile("/go/src/github.com/dinp/cert.pem")
	cert := string(cert_temp)
	if err != nil {
	     fmt.Print(err)
	}
	fmt.Println(cert)
	key_temp, err := ioutil.ReadFile("/go/src/github.com/dinp/key.pem")
	key := string(key_temp)
	if err != nil {
	     fmt.Print(err)
	}
	fmt.Println(key)
	ca_temp, err := ioutil.ReadFile("/go/src/github.com/dinp/ca.pem")
	ca := string(ca_temp)
	if err != nil {
	     fmt.Print(err)
	}
	fmt.Println(ca)*/
	client, err := docker.NewTLSClient(addr,"/go/src/github.com/dinp/cert.pem","/go/src/github.com/dinp/key.pem","/go/src/github.com/dinp/ca.pem")
	if err != nil {
		log.Println("docker.NewClient fail:", err)
		return
	}
	/*----------------------------edit by lianzhi20180706------------------------------------*/
	if err != nil {
		log.Println("[ERROR] docker.NewClient fail:", err)
		return
	}
	//动态加载用户指定端口
	var port string = fmt.Sprintf("%s/tcp", app.Port) //其实就是字符串类型
	
       str := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	//bytes := []byte(str)
	result := []byte{}
	//ran := rand.New(rand.NewSource(time.Now().UnixNano()))
	//for i := 0; i < 3; i++ {
	//	result = append(result, byte[ran.Intn(len(bytes))])
	//}
		bytess := []byte(str)
	res1 := []byte{}
	ra := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 32; i++ {
		res1 = append(res1, bytess[ra.Intn(len(bytess))])
	}
	for i := 0; i < 3; i++ {
		result = append(result, bytess[ra.Intn(len(bytess))])
	}
	name:=app.Name+string(result)
	binds :=strings.Split(app.Mount, ",")
	//binds := []string{"/home/lianzhi/data/"+name+":"+app.Mount}
	
		log.Println("app.Name", name)	
	opts := docker.CreateContainerOptions{
		Name:name,
		Config: &docker.Config{
			Memory: int64(app.Memory * 1024 * 1024),
			ExposedPorts: map[docker.Port]struct{}{
				docker.Port(port): {},
			},
			Image:        app.Image,
			AttachStdin:  false,
			AttachStdout: false,
			AttachStderr: false,
			Env:          BuildEnvArray(envVars),
			CPUShares:    int64(app.CPUShares),
		},
		HostConfig: &docker.HostConfig{
			Binds: binds,
			Privileged: true,
			PortBindings: map[string][]docker.PortBinding{
				port: []docker.PortBinding{docker.PortBinding{}},//"80/tcp"与port有什么区别呢?
			},
		},
	}

	container, err := client.CreateContainer(opts)
	if err != nil {
   	 log.Println(err)
	}

stmt, err := g.DB.Prepare("insert into ysy_app_container(id,app_id,con_id,con_name,con_volume,con_port)values(?,?,?,?,?,?)")
	if err != nil {
   	 log.Println(err)
	}
	log.Println("app.Id", app.Id)
	log.Println("container.ID",container.ID)
	log.Println("name",name)
	log.Println("app.Mount",app.Mount)
	log.Println("0","0")
	res,err:= stmt.Exec(string(res1),app.Id, container.ID,name,"/home/lianzhi/data/"+name+":"+app.Mount,"0")
	log.Println("result3", string(res1))
	if err != nil {
   	 log.Println(err)
	}
	 affect, err := res.RowsAffected()  
		if err != nil {
   	 log.Println(err)
	}
    fmt.Println(affect)
	//	session.Run("ll /root/dinp/data/"+name)


	if err != nil {
   	 log.Println(err)
	}
	if err != nil { 
		log.Printf("[ERROR] exec %s fail: %s", err)
		return 
	}
	if err != nil {
		if err == docker.ErrNoSuchImage {
			repos, tag := ParseRepositoryTag(app.Image)
			e := client.PullImage(docker.PullImageOptions{Repository: repos, Tag: tag}, docker.AuthConfiguration{})
			if e != nil {
				log.Println("[ERROR] pull image", app.Image, "fail:", e)
				return
			}

			// retry
			container, err = client.CreateContainer(opts)
			if err != nil {
				log.Println("[ERROR] retry create container fail:", err, "ip:", ip)
				g.UpdateAppStatus(app, model.AppStatus_CreateContainerFail)
				return
			}
		} else {
			log.Println("[ERROR] create container fail:", err, "ip:", ip)
			if err != nil && strings.Contains(err.Error(), "cannot connect") {
				g.DeleteNode(ip)
				g.RealState.DeleteByIp(ip)
				return
			}
			g.UpdateAppStatus(app, model.AppStatus_CreateContainerFail)
			return
		}
	}

	err = client.StartContainer(container.ID, &docker.HostConfig{
		//PortBindings: map[docker.Port][]docker.PortBinding{
		PortBindings: map[string][]docker.PortBinding{
			port: []docker.PortBinding{docker.PortBinding{}},
		},
	})
	session, err := connect("root", "Lin_1234", "10.17.0.244", 22)
	if err != nil {
   	 log.Fatal(err)
  	}
 	defer session.Close()
 	session.Stdout = os.Stdout
  	session.Stderr = os.Stderr
	session.Run("chmod -R  777  /home/lianzhi/data")
	log.Println("0","0")
	if err != nil {
		log.Println("[ERROR] docker.StartContainer fail:", err)
		g.UpdateAppStatus(app, model.AppStatus_StartContainerFail)
		return
	}

	if g.Config().Debug {
		log.Println("start container success:-)")
	}

}
