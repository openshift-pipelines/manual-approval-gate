# Installing Manual Approval Gate

NOTE:- You need to install [tektoncd/pipeline](https://github.com/tektoncd/pipeline/blob/main/docs/install.md)

1. Manual Approval Gate Installation
   *  On Kubernetes
       ```
       make apply
       ```
   * On Openshift
      ```
      make TARGET=openshift apply
      ```

2. Install a pipelineRun which has approval task as one of the task in the pipelin
   - For example 
     ```yaml
     apiVersion: tekton.dev/v1beta1
     kind: PipelineRun
     metadata:
       generateName: pr-custom-task-beta-
     spec:
       pipelineSpec:
         tasks:
           - name: before    
             taskSpec:
               steps:
                 - image: busybox
                   name: before
                   script: echo before wait
           - name: wait                     👈 Wait task with kind as `ApprovalTask`                           
             taskRef:
               apiVersion: openshift-pipelines.org/v1alpha1
               kind: ApprovalTask
             runAfter: ['before']
           - name: after
             taskSpec:
               steps:
                 - image: busybox
                   name: after
                   script: echo after wait
             runAfter: ['wait']
    
     ```
   Install the above pipelineRun
    
    ```shell
    kubectl create -f <pipeline.yaml>
    ```
   
    **NOTE** :- _Once the pipelineRun is started after the execution of first task is done, it will create a customRun for that approval task and the pipeline will be in wait state till it gets the approval from the user. The name of the approvalTask is the same of customRun which is created. 
As of today only `"true"` and `"false"` are supported. If user passes the approval as `"true"` then pipeline will proceed to execute the further tasks and if `"false"` is provided then in that case it will fail the pipeline_


3. Now start the api server
    ```shell
    go run ./cmd/approver/main.go
    ```
4. Now approve/reject the approval task using curl request

    ```shell
    curl --header "Content-Type: application/json" \                                                                                           
    --request POST \
    --data '{"approved":"false", "namespace":"default"}' \
    http://localhost:8000/approvaltask/<approvalTaskName>
    ```