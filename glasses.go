package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"

	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	//
	// Uncomment to load all auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth"
	//
	// Or uncomment to load specific auth plugins
	// _ "k8s.io/client-go/plugin/pkg/client/auth/azure"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	// _ "k8s.io/client-go/plugin/pkg/client/auth/openstack"
)

var (
	k8sHostname   string
	versionUrl    = "https://github.com/wakeful/glasses"
	version       = "dev"
	hostFile      = flag.String("host-file", "/etc/hosts", "host file location")
	writeHostFile = flag.Bool("write", false, "rewrite host file?")
	showVersion   = flag.Bool("version", false, "show version and exit")
	kubeconfig    = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
)

const (
	sectionStart = "# generated using k8s-ingress-hosts start #"
	sectionEnd   = "# generated using k8s-ingress-hosts end #\n"
)

func homeDir() string {
	if h := os.Getenv("HOME"); h != "" {
		return h
	}

	return os.Getenv("USERPROFILE")
}

type Rule struct {
	Domain  string
	Service string
}

func (r *Rule) String() string { return fmt.Sprintf("%s %s\t# %s", k8sHostname, r.Domain, r.Service) }

type HostsList []Rule

func (h HostsList) Len() int      { return len(h) }
func (h HostsList) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h HostsList) Less(i, j int) bool {
	return strings.ToLower(h[i].Domain) < strings.ToLower(h[j].Domain)
}

func k8sHost(config *rest.Config) string {
	u, err := url.Parse(config.Host)
	if err != nil {
		log.Fatalln(err.Error())
	}

	return u.Hostname()
}

func tryWriteToHostFile(hostEntries string) error {

	block := []byte(fmt.Sprintf("%s\n%s\n%s", sectionStart, hostEntries, sectionEnd))
	fileContent, err := ioutil.ReadFile(*hostFile)
	if err != nil {
		return err
	}

	re := regexp.MustCompile(fmt.Sprintf("(?ms)%s(.*)%s", sectionStart, sectionEnd))
	if re.Match(fileContent) {
		fileContent = re.ReplaceAll(fileContent, block)
	} else {
		fileContent = append(fileContent, block...)
	}

	if err := ioutil.WriteFile(*hostFile, fileContent, 0644); err != nil {
		return err
	}

	fmt.Println(hostEntries)
	return nil
}

func main() {
	flag.Parse()

	if *showVersion {
		fmt.Printf("Glasses\n url: %s\n version: %s", versionUrl, version)
		os.Exit(2)
	}

	fmt.Println("# reading k8s ingress resource...")
	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		log.Fatalln(err.Error())
	}

	k8sHostname = k8sHost(config)

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatalln(err.Error())
	}

	ingress, err := client.ExtensionsV1beta1().Ingresses("").List(metaV1.ListOptions{})
	if err != nil {
		log.Fatalln(err.Error())
	}

	var entries HostsList
	for _, elem := range ingress.Items {
		for _, rule := range elem.Spec.Rules {
			entries = append(entries, Rule{
				Domain:  rule.Host,
				Service: elem.Name,
			})
		}
	}

	sort.Sort(HostsList(entries))

	var hostEntries string
	for _, item := range entries {
		hostEntries = hostEntries + fmt.Sprintf("%s\n", item.String())
	}

	wBuffer := new(bytes.Buffer)
	writer := tabwriter.NewWriter(wBuffer, 0, 0, 2, ' ', 0)
	fmt.Fprint(writer, hostEntries)
	writer.Flush()

	if !*writeHostFile {
		fmt.Println(wBuffer.String())
		os.Exit(0)
	}

	if err := tryWriteToHostFile(wBuffer.String()); err != nil {
		log.Fatalln(err)
	}

}
