package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type configuration struct {
	Repo          string `json:"repo"`
	Token         string `json:"token"`
	Username      string `json:"username"`
	DefaultBranch string `json:"default_branch"`
	DefaultPath   string `json:"default_path"`
}

type requestFormat struct {
	M      string `json:"message"`
	C      string `json:"content"`
	SHA    string `json:"sha,omitempty"`
	Branch string `json:"branch,omitempty"`
	Path   string `json:"-"`
}

var (
	file   []byte
	config configuration
)

func main() {
	userConfig, err := os.UserConfigDir()
	if err != nil {
		fmt.Printf("Couldn't get user config path, %v\n", err)
	}
	configFilePath := fmt.Sprintf("%s/blog-tool/config.json", userConfig)

	// Subcommands
	configCommand := flag.NewFlagSet("config", flag.ExitOnError)
	pushCommand := flag.NewFlagSet("push", flag.ExitOnError)

	showConfig := configCommand.Bool("show", false, "Show current configs")

	customRepo := pushCommand.String("r", "", "Use specified repo")
	customUsername := pushCommand.String("u", "", "Use specified username")
	customToken := pushCommand.String("t", "", "Use specified token")
	remotePath := pushCommand.String("path", "", "Use specified path")
	branch := pushCommand.String("b", "", "Use specified branch")
	sha := pushCommand.String("sha", "", "Use specified sha")
	message := pushCommand.String("m", "", "Use specified message")

	if len(os.Args) == 1 {
		getConfig()
		pushCommand.Usage()
		configCommand.Usage()
		os.Exit(0)
	}

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "config":
			if len(os.Args) < 3 {
				fmt.Println("insufficient parameters supplied.")
				configCommand.Usage()
				os.Exit(1)
			}
			configCommand.Parse(os.Args[2:])
		case "push":
			if len(os.Args) < 3 {
				fmt.Println("insufficient parameters supplied.")
				pushCommand.Usage()
				os.Exit(1)
			}
			pushCommand.Parse(os.Args[2:])
		case "help":
			pushCommand.Usage()
			configCommand.Usage()
			os.Exit(0)
		}
	}

	if configCommand.Parsed() {
		if *showConfig {
			getConfig()
			jsonConfig, _ := json.MarshalIndent(config, "", "    ")
			fmt.Println(string(jsonConfig))
			os.Exit(0)
		}

		if os.Args[2] == "new" {
			os.Remove(configFilePath)
			firstTime(configFilePath)
			os.Exit(0)
		}

		if os.Args[2] == "delete" {
			os.Remove(configFilePath)
			os.Exit(0)
		}
	}

	if pushCommand.Parsed() {
		getConfig()
		fileName := os.Args[len(os.Args)-1]
		var requestBody requestFormat
		if *customRepo != "" {
			config.Repo = *customRepo
		}
		if *customUsername != "" {
			config.Username = *customUsername
		}
		if *customToken != "" {
			config.Token = *customToken
		}
		if *branch != "" {
			requestBody.Branch = *branch
		} else {
			fmt.Println("Using default branch")
			requestBody.Branch = config.DefaultBranch
		}
		if *sha != "" {
			requestBody.SHA = *sha
		}
		if *remotePath != "" {
			requestBody.Path = *remotePath
		} else {
			fmt.Println("Path not provided, using default directory...")
			requestBody.Path = fmt.Sprintf("%s%s", config.DefaultPath, os.Args[len(os.Args)-1])
		}
		if *message == "" {
			fmt.Println("Message not provided. Using ", fileName)
			*message = fileName
		}

		requestBody.M = *message

		fileContent, err := ioutil.ReadFile(fileName)
		if err != nil {
			fmt.Printf("couldn't read file, %v\n\nExiting...\n", err)
			os.Exit(1)
		}

		requestBody.C = base64.StdEncoding.Strict().EncodeToString(fileContent)
		requestBodyJSON, err := json.Marshal(requestBody)
		if err != nil {
			fmt.Printf("couldn't form request, %v\nExiting...\n", err)
		}

		req, err := http.NewRequest(http.MethodPut, (config.getAPIUrl(requestBody.Path)), bytes.NewBuffer(requestBodyJSON))
		if err != nil {
			panic(err)
		}

		req.Header.Set("Accept", "application/vnd.github.v3+json")
		req.SetBasicAuth(config.Username, config.Token)

		client := &http.Client{}
		response, err := client.Do(req)

		if err != nil {
			fmt.Printf("couldn't send request, %v\n", err)
			os.Exit(1)
		}

		if response.StatusCode > 250 {
			respMessage, _ := ioutil.ReadAll(response.Body)
			fmt.Printf("error received from github, %v\n", string(respMessage))
			os.Exit(1)
		}
		fmt.Println("success")
		os.Exit(0)
	}
	os.Exit(1)
}

func (c *configuration) getAPIUrl(path string) string {
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/contents%s", c.Username, c.Repo, path)
}

func getConfig() string {
	userConfig, err := os.UserConfigDir()

	if err != nil {
		fmt.Printf("couldn't get user config path, %v\n", err)
		os.Exit(1)
	}

	configFile := fmt.Sprintf("%s/blog-tool/config.json", userConfig)

	file, err = ioutil.ReadFile(configFile)

	if err != nil {
		fmt.Printf("Looks like this is your first time .Let's create a config file.\n")
		firstTime(configFile)
	}

	if config.Token == "" {
		if err = json.Unmarshal(file, &config); err != nil {
			fmt.Printf("config file is corrupt, %v\nLet's create a new one\n", err)
			firstTime(configFile)
		}
	}

	return configFile
}

func firstTime(filePath string) {
	fmt.Printf("Github username: ")
	fmt.Scan(&config.Username)
	fmt.Printf("Repo name: ")
	fmt.Scan(&config.Repo)
	fmt.Printf("Github Personal Token (can be created at https://github.com/settings/tokens [make sure repo permissions are given])\nToken: ")
	fmt.Scan(&config.Token)
	fmt.Printf("Default branch to use: ")
	fmt.Scan(&config.DefaultBranch)
	fmt.Printf("Default directory to use (For root use '/'): ")
	fmt.Scan(&config.DefaultPath)

	saveConfig(filePath)
}

func saveConfig(filePath string) {
	fileData, err := json.MarshalIndent(config, "", "    ")
	if err != nil {
		fmt.Printf("error while encoding config file, %v\n", err)
	}

	userConfig, _ := os.UserConfigDir()
	os.Mkdir(userConfig+"/blog-tool", 0755)
	if err != nil {
		fmt.Printf("error occurred, %v\n", err)
		return
	}

	if err = ioutil.WriteFile(filePath, fileData, 0644); err != nil {
		fmt.Printf("couldn't write config file, %v\nExiting....\n", err)
	}

}
