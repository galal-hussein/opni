IMAGE_NAME=sanjayrancher/drain-service
docker build .. -t $IMAGE_NAME -f ./Dockerfile

docker push $IMAGE_NAME
