package main

import (
	"log"
	"time"

	sdk "github.com/gaia-pipeline/gosdk"
)

// 1.启动时有自带的环境变量如{DateTime}之类
// 2.可以自定义变量在流水线运行时传递
// 3.每种任务有特定的配置
// 4.如果有输出则会在上下文一直传递
var jobs = sdk.Jobs{
	sdk.Job{
		Handler:     CreateUser,
		Title:       "Create DB User",
		Description: "Creates a database user with least privileged permissions.",
	},
	sdk.Job{
		Handler:     MigrateDB,
		Title:       "DB Migration",
		Description: "Imports newest test data dump and migrates to newest version.",
		DependsOn:   []string{"Create DB User"},
		Args: sdk.Arguments{
			sdk.Argument{
				Description: "Username for the database schema:",
				// TextFieldInp displays a text field in the UI.
				// You can also use sdk.TextAreaInp for text area,
				// sdk.BoolInp for boolean input.
				Type: sdk.TextFieldInp,
				Key:  "${username}",
			},
			sdk.Argument{
				Description: "Description for username:",
				// TextFieldInp displays a text field in the UI.
				// You can also use sdk.TextAreaInp for text area and
				// sdk.BoolInp for boolean input.
				Type: sdk.TextAreaInp,
				Key:  "usernamedesc",
			},
		},
		Interaction: &sdk.ManualInteraction{
			Description: "Enter username and description for the new user.",
			Type:        sdk.TextFieldInp,
			Value:       "test",
		},
	},
	sdk.Job{
		Handler:     CreateNamespace,
		Title:       "Create K8S Namespace",
		Description: "Creates a new Kubernetes namespace for the new test environment.",
		DependsOn:   []string{"DB Migration"},
	},
	sdk.Job{
		Handler:     CreateDeployment,
		Title:       "Create K8S Deployment",
		Description: "Creates a new Kubernetes deployment for the new test environment.",
		DependsOn:   []string{"Create K8S Namespace"},
	},
	sdk.Job{
		Handler:     CreateService,
		Title:       "Create K8S Service",
		Description: "Creates a new Kubernetes service for the new test environment.",
		DependsOn:   []string{"Create K8S Namespace"},
	},
	sdk.Job{
		Handler:     CreateIngress,
		Title:       "Create K8S Ingress",
		Description: "Creates a new Kubernetes ingress for the new test environment.",
		DependsOn:   []string{"Create K8S Namespace"},
	},
	sdk.Job{
		Handler:     Cleanup,
		Title:       "Clean up",
		Description: "Removes all temporary files.",
		DependsOn:   []string{"Create K8S Deployment", "Create K8S Service", "Create K8S Ingress"},
	},
}

func CreateUser(args sdk.Arguments) error {
	log.Println("CreateUser has been started!, args: ", args)

	// lets sleep to simulate that we do something
	time.Sleep(5 * time.Second)
	log.Println("CreateUser has been finished!")
	return nil
}

func MigrateDB(args sdk.Arguments) error {
	log.Println("MigrateDB has been started! args: ", args)

	// lets sleep to simulate that we do something
	time.Sleep(5 * time.Second)
	log.Println("MigrateDB has been finished!")
	return nil
}

func CreateNamespace(args sdk.Arguments) error {
	log.Println("CreateNamespace has been started! args: ", args)

	// lets sleep to simulate that we do something
	time.Sleep(5 * time.Second)
	log.Println("CreateNamespace has been finished!")
	return nil
}

func CreateDeployment(args sdk.Arguments) error {
	log.Println("CreateDeployment has been started! args: ", args)

	// lets sleep to simulate that we do something
	time.Sleep(5 * time.Second)
	log.Println("CreateDeployment has been finished!")
	return nil
}

func CreateService(args sdk.Arguments) error {
	log.Println("CreateService has been started!")

	// lets sleep to simulate that we do something
	time.Sleep(5 * time.Second)
	log.Println("CreateService has been finished!")
	return nil
}

func CreateIngress(args sdk.Arguments) error {
	log.Println("CreateIngress has been started!")

	// lets sleep to simulate that we do something
	time.Sleep(5 * time.Second)
	log.Println("CreateIngress has been finished!")
	return nil
}

func Cleanup(args sdk.Arguments) error {
	log.Println("Cleanup has been started!")

	// lets sleep to simulate that we do something
	time.Sleep(5 * time.Second)
	log.Println("Cleanup has been finished!")
	return nil
}

func main() {
	// Serve
	if err := sdk.Serve(jobs); err != nil {
		panic(err)
	}
}
