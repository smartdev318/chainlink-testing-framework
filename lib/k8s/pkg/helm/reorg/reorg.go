package reorg

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/smartcontractkit/chainlink-testing-framework/lib/k8s/client"
	"github.com/smartcontractkit/chainlink-testing-framework/lib/k8s/config"
	"github.com/smartcontractkit/chainlink-testing-framework/lib/k8s/environment"
	"github.com/smartcontractkit/chainlink-testing-framework/lib/utils/projectpath"
)

const (
	URLsKey            = "geth"
	TXNodesAppLabel    = "geth-ethereum-geth"
	MinerNodesAppLabel = "geth-ethereum-miner-node" // #nosec G101
)

type Props struct {
	NetworkName string `envconfig:"network_name"`
	NetworkType string `envconfig:"network_type"`
	Values      map[string]interface{}
}

type Chart struct {
	Name    string
	Path    string
	Version string
	Props   *Props
	Values  *map[string]interface{}
}

func (m Chart) IsDeploymentNeeded() bool {
	return true
}

func (m Chart) GetName() string {
	return m.Name
}

func (m Chart) GetProps() interface{} {
	return m.Props
}

func (m Chart) GetPath() string {
	return m.Path
}

func (m Chart) GetVersion() string {
	return m.Version
}

func (m Chart) GetValues() *map[string]interface{} {
	return m.Values
}

func (m Chart) GetLabels() map[string]string {
	return map[string]string{
		"chain.link/component": "reorg",
	}
}

func (m Chart) ExportData(e *environment.Environment) error {
	urls := make([]string, 0)
	httpURLs := make([]string, 0)
	networkName := strings.ReplaceAll(strings.ToLower(m.Props.NetworkName), " ", "-")
	minerPods, err := e.Client.ListPods(e.Cfg.Namespace, fmt.Sprintf("app=%s-ethereum-miner-node", networkName))
	if err != nil {
		return err
	}
	txPods, err := e.Client.ListPods(e.Cfg.Namespace, fmt.Sprintf("app=%s-ethereum-geth", networkName))
	if err != nil {
		return err
	}

	if len(txPods.Items) > 0 {
		for i := range txPods.Items {
			podName := fmt.Sprintf("%s-ethereum-geth:%d", networkName, i)
			txNodeLocalWS, err := e.Fwd.FindPort(podName, "geth", "ws-rpc").As(client.LocalConnection, client.WS)
			if err != nil {
				return err
			}
			txNodeLocalhttp, err := e.Fwd.FindPort(podName, "geth", "http-rpc").As(client.LocalConnection, client.HTTP)
			if err != nil {
				return err
			}
			if e.Cfg.InsideK8s {
				services, err := e.Client.ListServices(e.Cfg.Namespace, fmt.Sprintf("app=%s-ethereum-geth", networkName))
				if err != nil {
					return err
				}
				serviceURL := fmt.Sprintf("ws://%s:8546", services.Items[0].Name)
				urls = append(urls, serviceURL)
				log.Info().Str("wsURL", serviceURL).Msgf("Geth network (TX Node) - %d", i)
				httpURL := fmt.Sprintf("http://%s:8544", services.Items[0].Name)
				httpURLs = append(httpURLs, httpURL)
				log.Info().Str("httpURL", httpURL).Msgf("Geth network (TX Node) - %d", i)
			} else {
				urls = append(urls, txNodeLocalWS)
				log.Info().Str("URL", txNodeLocalWS).Msgf("Geth network (TX Node) - %d", i)
				httpURLs = append(httpURLs, txNodeLocalhttp)
				log.Info().Str("URL", txNodeLocalhttp).Msgf("Geth network (TX Node) - %d", i)
			}
		}
	}

	if len(minerPods.Items) > 0 {
		for i := range minerPods.Items {
			podName := fmt.Sprintf("%s-ethereum-miner-node:%d", networkName, i)
			minerNodeLocalWS, err := e.Fwd.FindPort(podName, "geth-miner", "ws-rpc-miner").As(client.LocalConnection, client.WS)
			if err != nil {
				return err
			}
			minerNodeInternalWs, err := e.Fwd.FindPort(podName, "geth-miner", "ws-rpc-miner").As(client.RemoteConnection, client.WS)
			if err != nil {
				return err
			}
			if e.Cfg.InsideK8s {
				urls = append(urls, minerNodeInternalWs)
				log.Info().Str("URL", minerNodeInternalWs).Msgf("Geth network (Miner Node) - %d", i)
			} else {
				urls = append(urls, minerNodeLocalWS)
				log.Info().Str("URL", minerNodeLocalWS).Msgf("Geth network (Miner Node) - %d", i)
			}
		}
	}

	e.URLs[m.Props.NetworkName] = urls
	e.URLs[m.Props.NetworkName+"_http"] = httpURLs
	return nil
}

func defaultProps() *Props {
	internalRepo := os.Getenv(config.EnvVarInternalDockerRepo)
	gethRepo := "ethereum/client-go"
	bootnodeRepo := "jpoon/bootnode-registrar"
	if internalRepo != "" {
		gethRepo = fmt.Sprintf("%s/ethereum/client-go", internalRepo)
		bootnodeRepo = fmt.Sprintf("%s/jpoon/bootnode-registrar", internalRepo)
	}
	return &Props{
		NetworkName: "geth",
		NetworkType: "geth-reorg",
		Values: map[string]interface{}{
			"imagePullPolicy": "IfNotPresent",
			"bootnode": map[string]interface{}{
				"replicas": "2",
				"image": map[string]interface{}{
					"repository": gethRepo,
					"tag":        "alltools-v1.10.25",
				},
			},
			"bootnodeRegistrar": map[string]interface{}{
				"replicas": "1",
				"image": map[string]interface{}{
					"repository": bootnodeRepo,
					"tag":        "v1.0.0",
				},
			},
			"geth": map[string]interface{}{
				"image": map[string]interface{}{
					"repository": gethRepo,
					"tag":        "v1.10.25",
				},
				"tx": map[string]interface{}{
					"replicas": "1",
					"service": map[string]interface{}{
						"type": "ClusterIP",
					},
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "2",
							"memory": "2Gi",
						},
						"limits": map[string]interface{}{
							"cpu":    "2",
							"memory": "2Gi",
						},
					},
				},
				"miner": map[string]interface{}{
					"replicas": "2",
					"account": map[string]interface{}{
						"secret": "",
					},
					"resources": map[string]interface{}{
						"requests": map[string]interface{}{
							"cpu":    "2",
							"memory": "2Gi",
						},
						"limits": map[string]interface{}{
							"cpu":    "2",
							"memory": "2Gi",
						},
					},
				},
				"genesis": map[string]interface{}{
					"networkId": "1337",
				},
			},
		},
	}
}

func New(props *Props) environment.ConnectedChart {
	return NewVersioned("", props)
}

// NewVersioned enables choosing a specific helm chart version
func NewVersioned(helmVersion string, props *Props) environment.ConnectedChart {
	targetProps := defaultProps()
	config.MustMerge(targetProps, props)
	config.MustMerge(&targetProps.Values, props.Values)
	chartPath := "chainlink-qa/ethereum"
	if b, err := strconv.ParseBool(os.Getenv(config.EnvVarLocalCharts)); err == nil && b {
		chartPath = fmt.Sprintf("%s/geth-reorg", projectpath.ChartsRoot)
	}
	return Chart{
		Name:    strings.ReplaceAll(strings.ToLower(targetProps.NetworkName), " ", "-"), // name cannot contain spaces
		Path:    chartPath,
		Values:  &targetProps.Values,
		Props:   targetProps,
		Version: helmVersion,
	}
}
