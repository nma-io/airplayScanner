#!/bin/zsh

# Clean up any existing resource.syso and versioninfo.json (only used for signed windows builds)
if [ -f resource.syso ]; then
    rm resource.syso versioninfo.json
fi

version=$(rg -aHILNoP 'version\s+=\s+"\K[^"]+' main.go) # Pull version info from maingo
echo $version > release/airplayScanner.version

# Build the releases
gobuilder -o release/airplayScanner.osx.arm .
gobuilder -o release/airplayScanner.osx.x86 --macosx86 .
gobuilder -o release/airplayScanner.linux.arm --linuxarm  .
gobuilder -o release/airplayScanner.linux.x86 --linux  .
gobuilder -o release/airplayScanner.linux.ish --ish  .
gobuilder -o release/airplayScanner.windows.x86 --winx86  . 
gobuilder -o release/airplayScanner.windows.arm --winarm  . 

if [ -f resource.syso ]; then
    rm resource.syso versioninfo.json
fi

for file in release/*; do
    base_filename=$(basename $file)
    aws s3 cp $file s3://disog-files/airplayScanner/$base_filename --acl public-read
done
echo "All files uploaded to S3."
echo "Build process completed."
