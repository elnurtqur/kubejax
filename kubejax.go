package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

type KubeConfig struct {
	APIVersion     string                 `yaml:"apiVersion"`
	Kind           string                 `yaml:"kind"`
	Clusters       []Cluster              `yaml:"clusters"`
	Contexts       []Context              `yaml:"contexts"`
	CurrentContext string                 `yaml:"current-context"`
	Users          []User                 `yaml:"users"`
	Preferences    map[string]interface{} `yaml:"preferences,omitempty"`
}

type Context struct {
	Name    string        `yaml:"name"`
	Context ContextDetail `yaml:"context"`
}

type ContextDetail struct {
	Cluster   string `yaml:"cluster"`
	User      string `yaml:"user"`
	Namespace string `yaml:"namespace,omitempty"`
}

type Cluster struct {
	Name    string        `yaml:"name"`
	Cluster ClusterDetail `yaml:"cluster"`
}

type ClusterDetail struct {
	CertificateAuthorityData string `yaml:"certificate-authority-data,omitempty"`
	CertificateAuthority     string `yaml:"certificate-authority,omitempty"`
	Server                   string `yaml:"server"`
	InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify,omitempty"`
}

type User struct {
	Name string     `yaml:"name"`
	User UserDetail `yaml:"user"`
}

type UserDetail struct {
	ClientCertificateData string                 `yaml:"client-certificate-data,omitempty"`
	ClientKeyData         string                 `yaml:"client-key-data,omitempty"`
	ClientCertificate     string                 `yaml:"client-certificate,omitempty"`
	ClientKey             string                 `yaml:"client-key,omitempty"`
	Token                 string                 `yaml:"token,omitempty"`
	Username              string                 `yaml:"username,omitempty"`
	Password              string                 `yaml:"password,omitempty"`
	AuthProvider          map[string]interface{} `yaml:"auth-provider,omitempty"`
	Exec                  map[string]interface{} `yaml:"exec,omitempty"`
}

type ConfigInfo struct {
	FilePath string
	Contexts []string
}

var (
	configDir       string
	interactiveMode bool
	listMode        bool
	currentMode     bool
	searchMode      bool
	outputConfig    string
	previousContext string
	currentContext  string
)

var productionKeywords = []string{"prd", "production"}
var productionExactKeywords = []string{"prod"}

const shellFunction = `# KUBEJAX shell function
kjx() {
    local temp_file=$(mktemp)
    local kjx_binary="%s"
    
    # Run kjx with output-config and pass all arguments
    if command "$kjx_binary" --output-config "$temp_file" "$@"; then
        # Check if temp file exists and has content
        if [ -f "$temp_file" ] && [ -s "$temp_file" ]; then
            local new_kubeconfig=$(cat "$temp_file")
            if [ -n "$new_kubeconfig" ]; then
                export KUBECONFIG="$new_kubeconfig"
                echo "KUBECONFIG exported: $KUBECONFIG"
            fi
        fi
    fi
    
    # Clean up temp file
    rm -f "$temp_file" 2>/dev/null
}`

func init() {
	homeDir, _ := os.UserHomeDir()
	configDir = filepath.Join(homeDir, ".kube", "configs")
}

func main() {
	var rootCmd = &cobra.Command{
		Use:   "kjx",
		Short: "KUBEJAX - Kubernetes Jump Across conteXts",
		Long:  `KUBEJAX: A lightning-fast tool to jump across contexts and namespaces in multiple kubeconfig files`,
		Run:   runContextSwitcher,
	}

	var nsCmd = &cobra.Command{
		Use:   "ns",
		Short: "Switch between namespaces",
		Long:  `Switch between namespaces in the current context`,
		Run:   runNamespaceSwitcher,
	}

	var shellInitCmd = &cobra.Command{
		Use:   "shell-init",
		Short: "Generate shell function for environment variable management",
		Long:  `Generate shell function that properly exports KUBECONFIG environment variable`,
		Run:   runShellInit,
	}

	var installCmd = &cobra.Command{
		Use:   "install",
		Short: "Install kjx shell function to your shell profile",
		Long:  `Install kjx shell function to your ~/.bashrc or ~/.zshrc file`,
		Run:   runInstall,
	}

	rootCmd.Flags().StringVarP(&configDir, "config-dir", "d", configDir, "Directory containing kubeconfig files")
	rootCmd.Flags().BoolVarP(&interactiveMode, "interactive", "i", false, "Interactive mode with fuzzy search")
	rootCmd.Flags().BoolVarP(&listMode, "list", "l", false, "List all available contexts")
	rootCmd.Flags().BoolVarP(&currentMode, "current", "c", false, "Show current context information")
	rootCmd.Flags().BoolVarP(&searchMode, "search", "s", false, "Search contexts by name")
	rootCmd.Flags().StringVar(&outputConfig, "output-config", "", "Output selected config path to file")

	nsCmd.Flags().StringVarP(&configDir, "config-dir", "d", configDir, "Directory containing kubeconfig files")
	nsCmd.Flags().BoolVarP(&interactiveMode, "interactive", "i", false, "Interactive mode")
	nsCmd.Flags().BoolVarP(&listMode, "list", "l", false, "List all available namespaces")
	nsCmd.Flags().BoolVarP(&currentMode, "current", "c", false, "Show current namespace information")
	nsCmd.Flags().BoolVarP(&searchMode, "search", "s", false, "Search namespaces by name")
	nsCmd.Flags().StringVar(&outputConfig, "output-config", "", "Output selected config path to file")

	rootCmd.AddCommand(nsCmd)
	rootCmd.AddCommand(shellInitCmd)
	rootCmd.AddCommand(installCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func isWordSeparator(char byte, separators []string) bool {
	charStr := string(char)
	for _, sep := range separators {
		if charStr == sep {
			return true
		}
	}
	return false
}

func isExactWordMatch(text, keyword string) bool {
	separators := []string{"-", "_", ".", " "}
	
	index := strings.Index(text, keyword)
	for index != -1 {
		startIndex := index
		endIndex := index + len(keyword)
		
		validStart := startIndex == 0 || isWordSeparator(text[startIndex-1], separators)
		validEnd := endIndex >= len(text) || isWordSeparator(text[endIndex], separators)
		
		if validStart && validEnd {
			return true
		}
		
		index = strings.Index(text[endIndex:], keyword)
		if index != -1 {
			index += endIndex
		}
	}
	
	return false
}

func isProductionEnvironment(contextName string) bool {
	lowerContext := strings.ToLower(contextName)
	
	for _, keyword := range productionExactKeywords {
		if isExactWordMatch(lowerContext, keyword) {
			return true
		}
	}
	
	for _, keyword := range productionKeywords {
		if strings.Contains(lowerContext, keyword) {
			return true
		}
	}
	
	return false
}

func isProductionConfigFile(configFilePath string) bool {
	fileName := strings.ToLower(filepath.Base(configFilePath))
	
	for _, keyword := range productionExactKeywords {
		if isExactWordMatch(fileName, keyword) {
			return true
		}
	}
	
	for _, keyword := range productionKeywords {
		if strings.Contains(fileName, keyword) {
			return true
		}
	}
	
	return false
}

func isProductionEnvironmentCombined(contextName, configFilePath string) bool {
	return isProductionEnvironment(contextName) || isProductionConfigFile(configFilePath)
}

func searchContexts(configInfos []ConfigInfo, searchTerm string) []string {
	var matches []string
	searchLower := strings.ToLower(searchTerm)
	
	for _, configInfo := range configInfos {
		for _, context := range configInfo.Contexts {
			if strings.Contains(strings.ToLower(context), searchLower) {
				matches = append(matches, context)
			}
		}
	}
	
	sort.Strings(matches)
	return matches
}

func searchNamespaces(namespaces []string, searchTerm string) []string {
	var matches []string
	searchLower := strings.ToLower(searchTerm)
	
	for _, namespace := range namespaces {
		if strings.Contains(strings.ToLower(namespace), searchLower) {
			matches = append(matches, namespace)
		}
	}
	
	sort.Strings(matches)
	return matches
}

func interactiveContextSearch(configInfos []ConfigInfo) error {
	var allContexts []string
	var contextFileMap = make(map[string]string)
	
	for _, configInfo := range configInfos {
		for _, context := range configInfo.Contexts {
			allContexts = append(allContexts, context)
			contextFileMap[context] = configInfo.FilePath
		}
	}
	
	if len(allContexts) == 0 {
		return fmt.Errorf("no contexts available")
	}
	
	searcher := promptui.Select{
		Label: "Search and select context (type to filter)",
		Items: allContexts,
		Size:  15,
		Searcher: func(input string, index int) bool {
			context := allContexts[index]
			name := strings.Replace(strings.ToLower(context), " ", "", -1)
			input = strings.Replace(strings.ToLower(input), " ", "", -1)
			return strings.Contains(name, input)
		},
	}
	
	_, result, err := searcher.Run()
	if err != nil {
		return err
	}
	
	filePath := contextFileMap[result]
	
	if isProductionEnvironmentCombined(result, filePath) {
		showProductionWarning(result, filePath)
	}
	
	return setKubeConfig(filePath, result)
}

func interactiveNamespaceSearch() error {
	namespaces, err := getLiveNamespaces()
	if err != nil {
		return fmt.Errorf("could not get namespaces: %v", err)
	}
	
	if len(namespaces) == 0 {
		return fmt.Errorf("no namespaces found")
	}
	
	searcher := promptui.Select{
		Label: "Search and select namespace (type to filter)",
		Items: namespaces,
		Size:  15,
		Searcher: func(input string, index int) bool {
			namespace := namespaces[index]
			name := strings.Replace(strings.ToLower(namespace), " ", "", -1)
			input = strings.Replace(strings.ToLower(input), " ", "", -1)
			return strings.Contains(name, input)
		},
	}
	
	_, result, err := searcher.Run()
	if err != nil {
		return err
	}
	
	currentConfig := os.Getenv("KUBECONFIG")
	if currentConfig == "" {
		homeDir, _ := os.UserHomeDir()
		currentConfig = filepath.Join(homeDir, ".kube", "config")
	}
	
	kubeconfig, err := loadKubeConfig(currentConfig)
	if err != nil {
		return err
	}
	
	return switchToNamespace(result, kubeconfig, currentConfig)
}

func showProductionWarning(contextName, configFilePath string) {
	fmt.Println("⚠️  WARNING: PRODUCTION ENVIRONMENT DETECTED!")
	fmt.Printf("🔴 You are selecting context: '%s'\n", contextName)
	
	if isProductionEnvironment(contextName) {
		fmt.Printf("🔴 Context name contains production keywords\n")
	}
	if isProductionConfigFile(configFilePath) {
		fmt.Printf("🔴 Config file '%s' contains production keywords\n", filepath.Base(configFilePath))
	}
	
	fmt.Println("🔴 This appears to be a PRODUCTION cluster.")
	fmt.Println("🔴 Please be extra careful with any changes!")
	fmt.Println()
}

func getCurrentContextInfo() (contextName, configFile, clusterName, namespace string) {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homeDir, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(homeDir, ".kube", "config")
	}

	config, err := loadKubeConfig(kubeconfig)
	if err != nil {
		return "", "", "", ""
	}

	contextName = config.CurrentContext
	configFile = filepath.Base(kubeconfig)

	for _, ctx := range config.Contexts {
		if ctx.Name == contextName {
			clusterName = ctx.Context.Cluster
			namespace = ctx.Context.Namespace
			if namespace == "" {
				namespace = "default"
			}
			break
		}
	}

	return contextName, configFile, clusterName, namespace
}

func runShellInit(cmd *cobra.Command, args []string) {
	execPath, err := os.Executable()
	if err != nil {
		execPath = "kjx"
	}
	
	fmt.Printf(shellFunction, execPath)
}

func runInstall(cmd *cobra.Command, args []string) {
	shell := os.Getenv("SHELL")
	var profileFile string
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error: Could not get home directory: %v\n", err)
		return
	}
	
	if strings.Contains(shell, "zsh") {
		profileFile = filepath.Join(homeDir, ".zshrc")
	} else if strings.Contains(shell, "bash") {
		profileFile = filepath.Join(homeDir, ".bashrc")
	} else {
		fmt.Println("Unsupported shell. Please manually add the shell function to your profile.")
		fmt.Println("Run 'kjx shell-init' to get the function code.")
		return
	}
	
	execPath, err := os.Executable()
	if err != nil {
		execPath = "kjx"
	}
	
	functionCode := fmt.Sprintf("\n# KUBEJAX shell function (auto-generated)\n%s\n", fmt.Sprintf(shellFunction, execPath))
	
	if _, err := os.Stat(profileFile); err == nil {
		content, err := ioutil.ReadFile(profileFile)
		if err == nil && strings.Contains(string(content), "KUBEJAX shell function") {
			fmt.Printf("KUBEJAX shell function already exists in %s\n", profileFile)
			return
		}
	}
	
	f, err := os.OpenFile(profileFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error: Could not open %s: %v\n", profileFile, err)
		return
	}
	defer f.Close()
	
	if _, err := f.WriteString(functionCode); err != nil {
		fmt.Printf("Error: Could not write to %s: %v\n", profileFile, err)
		return
	}
	
	fmt.Printf("✅ KUBEJAX shell function installed to %s\n", profileFile)
	fmt.Printf("Please run: source %s\n", profileFile)
	fmt.Println("Or restart your shell to use the function.")
}

func runContextSwitcher(cmd *cobra.Command, args []string) {
	if currentMode {
		showCurrentContextInfo()
		return
	}

	configInfos, err := loadAllKubeConfigs()
	if err != nil {
		fmt.Printf("Error loading kubeconfigs: %v\n", err)
		return
	}

	if len(configInfos) == 0 {
		fmt.Printf("No kubeconfig files found in %s\n", configDir)
		return
	}

	currentContext = getCurrentContext()

	if searchMode {
		if len(args) > 0 {
			searchTerm := args[0]
			matches := searchContexts(configInfos, searchTerm)
			if len(matches) == 0 {
				fmt.Printf("No contexts found matching '%s'\n", searchTerm)
				return
			}
			
			fmt.Printf("Contexts matching '%s':\n", searchTerm)
			for i, match := range matches {
				marker := "  "
				if match == currentContext {
					marker = "🔹"
				}
				
				var matchFilePath string
				for _, configInfo := range configInfos {
					for _, context := range configInfo.Contexts {
						if context == match {
							matchFilePath = configInfo.FilePath
							break
						}
					}
					if matchFilePath != "" {
						break
					}
				}
				
				prodIndicator := ""
				if isProductionEnvironmentCombined(match, matchFilePath) {
					prodIndicator = " 🔴"
				}
				fmt.Printf("%d) %s %s%s\n", i+1, marker, match, prodIndicator)
			}
			
			if len(matches) == 1 {
				fmt.Printf("\nOnly one match found. Switching to '%s'...\n", matches[0])
				
				var matchFilePath string
				for _, configInfo := range configInfos {
					for _, context := range configInfo.Contexts {
						if context == matches[0] {
							matchFilePath = configInfo.FilePath
							break
						}
					}
					if matchFilePath != "" {
						break
					}
				}
				
				if isProductionEnvironmentCombined(matches[0], matchFilePath) {
					showProductionWarning(matches[0], matchFilePath)
				}
				if err := switchToContext(matches[0], configInfos); err != nil {
					fmt.Printf("Error switching context: %v\n", err)
				}
			}
			return
		} else {
			if err := interactiveContextSearch(configInfos); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			return
		}
	}

	if listMode {
		listAllContexts(configInfos)
		return
	}

	if len(args) == 0 || interactiveMode {
		if err := interactiveContextSelect(configInfos); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		return
	}

	contextName := args[0]
	if contextName == "-" {
		if previousContext == "" {
			fmt.Println("No previous context available")
			return
		}
		contextName = previousContext
	}

	if isProductionEnvironment(contextName) {
		var filePath string
		for _, configInfo := range configInfos {
			for _, context := range configInfo.Contexts {
				if context == contextName {
					filePath = configInfo.FilePath
					break
				}
			}
			if filePath != "" {
				break
			}
		}
		showProductionWarning(contextName, filePath)
	}

	if err := switchToContext(contextName, configInfos); err != nil {
		fmt.Printf("Error switching context: %v\n", err)
	}
}

func showCurrentContextInfo() {
	contextName, configFile, clusterName, namespace := getCurrentContextInfo()
	
	if contextName == "" {
		fmt.Println("❌ No current context found or invalid kubeconfig")
		return
	}

	fmt.Println("📍 Current Kubernetes Context Information:")
	fmt.Println("=" + strings.Repeat("=", 45))
	fmt.Printf("🔹 Context: %s\n", contextName)
	fmt.Printf("📁 Config File: %s\n", configFile)
	fmt.Printf("🏗️  Cluster: %s\n", clusterName)
	fmt.Printf("📦 Namespace: %s\n", namespace)
	
	currentKubeconfig := os.Getenv("KUBECONFIG")
	if currentKubeconfig == "" {
		homeDir, _ := os.UserHomeDir()
		currentKubeconfig = filepath.Join(homeDir, ".kube", "config")
	}
	
	isProdContext := isProductionEnvironment(contextName) || isProductionEnvironment(clusterName)
	isProdFile := isProductionConfigFile(currentKubeconfig)
	
	if isProdContext || isProdFile {
		fmt.Println()
		fmt.Println("⚠️  PRODUCTION ENVIRONMENT DETECTED!")
		
		if isProdContext {
			fmt.Printf("🔴 Context/Cluster '%s' appears to be a production environment\n", contextName)
		}
		if isProdFile {
			fmt.Printf("🔴 Config file '%s' appears to be a production environment\n", configFile)
		}
		
		fmt.Println("🔴 Please be extra careful with any operations!")
	}
	
	fmt.Printf("\n💾 KUBECONFIG: %s\n", currentKubeconfig)
}

func runNamespaceSwitcher(cmd *cobra.Command, args []string) {
	if currentMode {
		showCurrentNamespaceInfo()
		return
	}

	currentConfig := os.Getenv("KUBECONFIG")
	if currentConfig == "" {
		homeDir, _ := os.UserHomeDir()
		currentConfig = filepath.Join(homeDir, ".kube", "config")
	}

	kubeconfig, err := loadKubeConfig(currentConfig)
	if err != nil {
		fmt.Printf("Error loading current kubeconfig: %v\n", err)
		return
	}

	if searchMode {
		if len(args) > 0 {
			searchTerm := args[0]
			namespaces, err := getLiveNamespaces()
			if err != nil {
				fmt.Printf("Error getting namespaces: %v\n", err)
				return
			}
			
			matches := searchNamespaces(namespaces, searchTerm)
			if len(matches) == 0 {
				fmt.Printf("No namespaces found matching '%s'\n", searchTerm)
				return
			}
			
			fmt.Printf("Namespaces matching '%s':\n", searchTerm)
			for i, match := range matches {
				fmt.Printf("%d) %s\n", i+1, match)
			}
			
			if len(matches) == 1 {
				fmt.Printf("\nOnly one match found. Switching to namespace '%s'...\n", matches[0])
				if err := switchToNamespace(matches[0], kubeconfig, currentConfig); err != nil {
					fmt.Printf("Error switching namespace: %v\n", err)
				}
			}
			return
		} else {
			if err := interactiveNamespaceSearch(); err != nil {
				fmt.Printf("Error: %v\n", err)
			}
			return
		}
	}

	if listMode {
		namespaces, err := getLiveNamespaces()
		if err != nil {
			fmt.Printf("Error getting namespaces: %v\n", err)
			return
		}
		
		fmt.Println("Available namespaces in current cluster:")
		for _, ns := range namespaces {
			fmt.Printf("  %s\n", ns)
		}
		return
	}

	if len(args) == 0 || interactiveMode {
		if err := interactiveNamespaceSelect(kubeconfig); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		return
	}

	namespace := args[0]
	if namespace == "-" {
		fmt.Println("Previous namespace switching not implemented yet")
		return
	}

	namespaces, err := getLiveNamespaces()
	if err != nil {
		fmt.Printf("Warning: Could not verify namespace exists: %v\n", err)
		fmt.Printf("Switching to namespace '%s' anyway...\n", namespace)
	} else {
		found := false
		for _, ns := range namespaces {
			if ns == namespace {
				found = true
				break
			}
		}
		if !found {
			fmt.Printf("Warning: Namespace '%s' not found in cluster\n", namespace)
			fmt.Printf("Available namespaces: %s\n", strings.Join(namespaces, ", "))
			return
		}
	}

	if err := switchToNamespace(namespace, kubeconfig, currentConfig); err != nil {
		fmt.Printf("Error switching namespace: %v\n", err)
	}
}

func showCurrentNamespaceInfo() {
	contextName, configFile, clusterName, namespace := getCurrentContextInfo()
	
	if contextName == "" {
		fmt.Println("❌ No current context found or invalid kubeconfig")
		return
	}

	fmt.Println("📦 Current Kubernetes Namespace Information:")
	fmt.Println("=" + strings.Repeat("=", 48))
	fmt.Printf("📦 Current Namespace: %s\n", namespace)
	fmt.Printf("🔹 Context: %s\n", contextName)
	fmt.Printf("🏗️  Cluster: %s\n", clusterName)
	fmt.Printf("📁 Config File: %s\n", configFile)

	currentKubeconfig := os.Getenv("KUBECONFIG")
	if currentKubeconfig == "" {
		homeDir, _ := os.UserHomeDir()
		currentKubeconfig = filepath.Join(homeDir, ".kube", "config")
	}
	
	isProdContext := isProductionEnvironment(contextName) || isProductionEnvironment(clusterName)
	isProdFile := isProductionConfigFile(currentKubeconfig)
	
	if isProdContext || isProdFile {
		fmt.Println()
		fmt.Println("⚠️  PRODUCTION ENVIRONMENT DETECTED!")
		fmt.Printf("🔴 You are working in a production environment\n")
		fmt.Printf("🔴 Current namespace: '%s'\n", namespace)
		
		if isProdContext {
			fmt.Printf("🔴 Context/Cluster contains production keywords\n")
		}
		if isProdFile {
			fmt.Printf("🔴 Config file contains production keywords\n")
		}
		
		fmt.Println("🔴 Please be extra careful with any operations!")
	}
}

func loadAllKubeConfigs() ([]ConfigInfo, error) {
	var configInfos []ConfigInfo

	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("config directory does not exist: %s", configDir)
	}

	files, err := ioutil.ReadDir(configDir)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		filePath := filepath.Join(configDir, file.Name())

		if strings.HasPrefix(file.Name(), ".") ||
			strings.HasSuffix(file.Name(), ".log") ||
			strings.HasSuffix(file.Name(), ".txt") ||
			strings.HasSuffix(file.Name(), ".md") {
			continue
		}

		kubeconfig, err := loadKubeConfig(filePath)
		if err != nil {
			fmt.Printf("Warning: Could not load %s: %v\n", file.Name(), err)
			continue
		}

		var contexts []string
		for _, ctx := range kubeconfig.Contexts {
			contexts = append(contexts, ctx.Name)
		}

		if len(contexts) > 0 {
			configInfos = append(configInfos, ConfigInfo{
				FilePath: filePath,
				Contexts: contexts,
			})
		}
	}

	return configInfos, nil
}

func getLiveNamespaces() ([]string, error) {
	cmd := exec.Command("kubectl", "get", "namespaces", "-o", "name", "--no-headers")
	
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute kubectl: %v", err)
	}

	var namespaces []string
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	
	for _, line := range lines {
		if line != "" {
			ns := strings.TrimPrefix(line, "namespace/")
			if ns != "" {
				namespaces = append(namespaces, ns)
			}
		}
	}

	sort.Strings(namespaces)
	
	return namespaces, nil
}

func loadKubeConfig(filePath string) (*KubeConfig, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var kubeconfig KubeConfig
	if err := yaml.Unmarshal(data, &kubeconfig); err != nil {
		return nil, err
	}

	return &kubeconfig, nil
}

func getCurrentContext() string {
	kubeconfig := os.Getenv("KUBECONFIG")
	if kubeconfig == "" {
		homeDir, _ := os.UserHomeDir()
		kubeconfig = filepath.Join(homeDir, ".kube", "config")
	}

	config, err := loadKubeConfig(kubeconfig)
	if err != nil {
		return ""
	}

	return config.CurrentContext
}

func listAllContexts(configInfos []ConfigInfo) {
	fmt.Printf("Available contexts from %s:\n\n", configDir)

	for _, configInfo := range configInfos {
		fileName := filepath.Base(configInfo.FilePath)
		fmt.Printf("📁 %s:\n", fileName)

		for _, context := range configInfo.Contexts {
			marker := "  "
			if context == currentContext {
				marker = "🔹"
			}
			
			prodIndicator := ""
			if isProductionEnvironmentCombined(context, configInfo.FilePath) {
				prodIndicator = " 🔴"
			}
			
			fmt.Printf("%s %s%s\n", marker, context, prodIndicator)
		}
		fmt.Println()
	}
	
	fmt.Println("Legend:")
	fmt.Println("🔹 = Current context")
	fmt.Println("🔴 = Production environment (context name or config file)")
}

func interactiveContextSelect(configInfos []ConfigInfo) error {
	var items []string
	var contextMap = make(map[string]string)

	for _, configInfo := range configInfos {
		fileName := filepath.Base(configInfo.FilePath)
		for _, context := range configInfo.Contexts {
			prodIndicator := ""
			if isProductionEnvironmentCombined(context, configInfo.FilePath) {
				prodIndicator = " 🔴"
			}
			display := fmt.Sprintf("%s (%s)%s", context, fileName, prodIndicator)
			items = append(items, display)
			contextMap[display] = configInfo.FilePath
		}
	}

	if len(items) == 0 {
		return fmt.Errorf("no contexts available")
	}

	sort.Strings(items)

	prompt := promptui.Select{
		Label: "Select context (type to search/filter)",
		Items: items,
		Size:  10,
		Searcher: func(input string, index int) bool {
			item := items[index]
			contextName := strings.Split(item, " (")[0]
			
			searchTarget := strings.Replace(strings.ToLower(contextName), " ", "", -1)
			displayTarget := strings.Replace(strings.ToLower(item), " ", "", -1)
			searchInput := strings.Replace(strings.ToLower(input), " ", "", -1)
			
			return strings.Contains(searchTarget, searchInput) || strings.Contains(displayTarget, searchInput)
		},
	}

	_, result, err := prompt.Run()
	if err != nil {
		return err
	}

	contextName := strings.Split(result, " (")[0]
	filePath := contextMap[result]

	if isProductionEnvironmentCombined(contextName, filePath) {
		showProductionWarning(contextName, filePath)
	}

	return setKubeConfig(filePath, contextName)
}

func interactiveNamespaceSelect(kubeconfig *KubeConfig) error {
	namespaces, err := getLiveNamespaces()
	if err != nil {
		fmt.Printf("Warning: Could not get live namespaces (%v), using defaults\n", err)
		namespaces = []string{
			"default",
			"kube-system",
			"kube-public",
			"kube-node-lease",
		}
	}

	if len(namespaces) == 0 {
		return fmt.Errorf("no namespaces found")
	}

	prompt := promptui.Select{
		Label: "Select namespace (type to search/filter)",
		Items: namespaces,
		Size:  15,
		Searcher: func(input string, index int) bool {
			namespace := namespaces[index]
			searchTarget := strings.Replace(strings.ToLower(namespace), " ", "", -1)
			searchInput := strings.Replace(strings.ToLower(input), " ", "", -1)
			return strings.Contains(searchTarget, searchInput)
		},
	}

	_, result, err := prompt.Run()
	if err != nil {
		return err
	}

	currentConfig := os.Getenv("KUBECONFIG")
	if currentConfig == "" {
		homeDir, _ := os.UserHomeDir()
		currentConfig = filepath.Join(homeDir, ".kube", "config")
	}

	return switchToNamespace(result, kubeconfig, currentConfig)
}

func switchToContext(contextName string, configInfos []ConfigInfo) error {
	for _, configInfo := range configInfos {
		for _, context := range configInfo.Contexts {
			if context == contextName {
				return setKubeConfig(configInfo.FilePath, contextName)
			}
		}
	}

	return fmt.Errorf("context '%s' not found", contextName)
}

func setKubeConfig(filePath, contextName string) error {
	previousContext = currentContext

	tempFile := "/tmp/kjx-config"
	if outputConfig != "" {
		tempFile = outputConfig
	}
	
	err := ioutil.WriteFile(tempFile, []byte(filePath), 0644)
	if err != nil {
		return fmt.Errorf("failed to write config path to file: %v", err)
	}

	if err := os.Setenv("KUBECONFIG", filePath); err != nil {
		return err
	}

	originalData, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}

	var rawConfig interface{}
	if err := yaml.Unmarshal(originalData, &rawConfig); err != nil {
		return err
	}

	if configMap, ok := rawConfig.(map[interface{}]interface{}); ok {
		configMap["current-context"] = contextName
	}

	data, err := yaml.Marshal(rawConfig)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(filePath, data, 0644); err != nil {
		return err
	}

	fmt.Printf("Switched to context '%s' in %s\n", contextName, filepath.Base(filePath))
	
	if isProductionEnvironment(contextName) {
		fmt.Println("🔴 You are now connected to a PRODUCTION environment!")
		fmt.Println("🔴 Please be extra careful with your operations!")
	}
	
	if outputConfig == "" {
		fmt.Printf("🔄 To export KUBECONFIG to your shell, run: export KUBECONFIG=%s\n", filePath)
		fmt.Println("💡 Or use shell integration with: kjx install && source ~/.zshrc")
	}
	
	return nil
}

func switchToNamespace(namespace string, kubeconfig *KubeConfig, configPath string) error {
	originalData, err := ioutil.ReadFile(configPath)
	if err != nil {
		return err
	}

	var rawConfig interface{}
	if err := yaml.Unmarshal(originalData, &rawConfig); err != nil {
		return err
	}

	if configMap, ok := rawConfig.(map[interface{}]interface{}); ok {
		if contextsInterface, exists := configMap["contexts"]; exists {
			if contexts, ok := contextsInterface.([]interface{}); ok {
				for _, ctxInterface := range contexts {
					if ctx, ok := ctxInterface.(map[interface{}]interface{}); ok {
						if nameInterface, exists := ctx["name"]; exists {
							if name, ok := nameInterface.(string); ok && name == kubeconfig.CurrentContext {
								if contextDetailInterface, exists := ctx["context"]; exists {
									if contextDetail, ok := contextDetailInterface.(map[interface{}]interface{}); ok {
										contextDetail["namespace"] = namespace
									}
								}
								break
							}
						}
					}
				}
			}
		}
	}

	data, err := yaml.Marshal(rawConfig)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(configPath, data, 0644); err != nil {
		return err
	}

	fmt.Printf("Switched to namespace '%s'\n", namespace)
	
	contextName := kubeconfig.CurrentContext
	if isProductionEnvironment(contextName) {
		fmt.Printf("🔴 You are working in namespace '%s' in a PRODUCTION environment!\n", namespace)
		fmt.Println("🔴 Please be extra careful with your operations!")
	}
	
	return nil
}