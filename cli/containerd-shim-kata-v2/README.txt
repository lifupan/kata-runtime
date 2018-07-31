1. To build the containerd-shim-kata-v2 binary file, please cd 
   into this directory and then run: go build

2. To use containerd-shim-kata-v2 to run a container:
   A: build the latest containerd with the following steps:
      $go get github.com/containerd/containerd
      $cd <containerd src directory> ; make; sudo make install

   B: start containerd daemon:
      $sudo containerd 

   C: deploy the shim binary containerd-shim-kata-v2 into $PATH
   
   D:
      $sudo ctr run --runtime containerd.shim.kata.v2 --rm -t <container image> <name> <cmd>
