
faas-cli login --username admin --password admin --gateway=10.100.12.143:8080

docker buildx build --platform linux/arm64 \
  -t sarveshwaran736479/face_blur:latest \
  -f Dockerfile . --push

faas-cli deploy --gateway=10.100.12.143:8080

base64 samples/nasa.jpg >image.base64

faas-cli invoke pigo-faceblur --gateway=http://10.100.12.143:8080 < image.base64 > output.txt

base64 -d output/output.base64  > output.jpeg

faas-cli logs pigo-faceblur --gateway=10.100.12.143:8080
