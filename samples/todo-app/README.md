# TODO Application Sample

This sample illustrates how Kdo can be used to develop a microservice version of the celebrated TODO application, adapted from code provided by [TodoMVC](http://todomvc.com).

Most sample TODO applications are composed of a frontend and some kind of backend persistent storage. This extended sample adds a statistics component and breaks the application into a number of microservices, specifically:

- The frontend uses a Mongo database to persist TODO items;
- The frontend writes add, complete and delete events to a RabbitMQ queue;
- A statistics worker receives events from the RabbitMQ queue and updates a Redis cache;
- A statistics API exposes the cached statistics for the frontend to show.

In all, this extended TODO application is composed of six inter-related components.

## Prerequisites

- Git CLI
- Docker CLI (no daemon required)
- Kubectl CLI (connected to a cluster)
- Kdo CLI (get latest release [here](https://github.com/stepro/kdo/releases))
- Visual Studio Code (optional)

## Clone this repository

We'll be working on code in this repository, so clone it:

```
git clone https://github.com/stepro/kdo
```

Then, change to the directory for this sample:

```
cd kdo/samples/todo-app
```

## Deploy the application

First, create a namespace for the sample:

```
kubectl create namespace kdo-todo-app
```

Then, apply the deployment manifest:

```
kubectl apply -n kdo-todo-app -f deployment.yaml
```

This is a simple deployment that exposes the frontend using a service of type `LoadBalancer`. Wait for all the pods to be running and for the external IP of the `frontend` service to become available.

Browse to the application using the external IP and give it a spin. As you add, complete and delete todos, notice that the stats page updates with the expected metrics.

## Develop changes to the frontend

Start by changing to the `frontend` directory, and then run:

```
kdo -n kdo-todo-app -c svc/frontend -p 3000:80 .
```

This builds an image from the current directory and runs it in a new pod configured like the existing frontend service. It also allows access to this version of the frontend through `localhost:3000`.

Open this URL in your browser and play with the app. You will see log output from the running command in the terminal. Notice that because it inherits the configuration of the existing frontend service, it is able to connect to all of its upstream services (the Mongo database, statistics API and statistics queue) without having to worry about complex deployment configuration.

Hit Ctrl+C in the terminal to end the current command. This deletes the temporary pod that was created to run this version of the frontend.

Now let's make a visible change to the frontend. Open the `components/TodoApp.js`, and search for the text "`What needs to be done`". Change this to "`What REALLY needs to be done`" and save. Now run the same kdo command again:

```
kdo -n kdo-todo-app -c svc/frontend -p 3000:80 .
```

This will build an updated image and run it as before. You can observe the changes by refreshing your browser (it also may update itself automatically).

Up to this point, we've been working in an isolated pod that doesn't replace the baseline instance of the frontend. You can validate this by browsing to the external IP and observing that it does not show the changes we made. To change this, let's Ctrl+C and add one more parameter to our kdo command:

```
kdo -n kdo-todo-app -c svc/frontend -R -p 3000:80 .
```

The `-R` (or `--replace`) flag tells kdo to temporarily replace the frontend service with the pod we're working on. Now if you browse to the external IP, you'll see the change. Ctrl+C to end, and it quickly reverts back to the baseline pod.

Kdo can be configured to enable rapid and effective in-cluster development and debugging. This sample has been pre-configured with both launch and attach debug configurations for Visual Studio Code. To give these a try, open the `frontend` directory in Visual Studio Code and press F5. These configurations have been set up to temporarily replace the baseline instance of the frontend, so you can use the external IP to hit breakpoints and so on.

    Note: before closing Visual Studio Code, make sure to press Ctrl+C on the long-running Kdo Prelaunch or Preattach task to revert the state of the cluster. Issue #25 will remove this requirement.

## Play around with other components

The statistics API and worker components have also been pre-configured for an optimized in-cluster development and debugging experience in Visual Studio Code. Try opening the `stats-api` and/or `stats-worker` directories and observe how Kdo makes it easy to develop one part of a larger microservice application deployed to Kubernetes without needing to worry about complex deployment configuration.

## Clean up

To clean up the assets produced by this sample, simply run:

```
kubectl delete namespace kdo-todo-app
```

To clean up kdo server components that were automatically installed, run:

```
kdo --uninstall
```
