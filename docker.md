docker build -t altair .
docker run -v /mnt/altair:/var/rinha altair
time docker run --memory=6m --cpus=1 -v /mnt/altair:/var/rinha altair
vim /mnt/altair/source.rinha.json