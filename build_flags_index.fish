set -x dataset (curl 'https://raw.githubusercontent.com/lukes/ISO-3166-Countries-with-Regional-Codes/master/all/all.json')
for code in (ls ../img/ | sed 's/\\(..\\)\.png/\1/' | tr '[:lower:]' '[:upper:]')
    set name (echo $dataset | jq --raw-output ".[] | select(.[\"alpha-2\"] == \"$code\") | .name")

    set lcode (echo $code | tr '[:upper:]' '[:lower:]')
    set ename (echo $name | sed -e 's/ /\\\\ /g')
    set ename (echo $ename | sed -e 's/(/\\\\(/g')
    set ename (echo $ename | sed -e 's/)/\\\\)/g')
    set ename (echo $ename | sed -e 's/\'/\\\\\'/g')
    set cmd "echo http://d1tefi9crrjlgi.cloudfront.net/$lcode.png > ../flags/$ename"
    echo $cmd
    bash -c $cmd
end
