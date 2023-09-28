<img src="./altair.png" width="150" height="150">

## Descrição
Altair Spankabytes é um simples interpretador feito exclusivamente para rodar a linguagem criada para a rinha de compiladores. Ele é baseado em tree-walking interpreter e é feito apenas em Go 1.21 com suas libs internas.

## Funcionalidades
- [x] Shadowing
- [x] Usa int64 como padrão e faz conversão automática em runtime para bigint (suportando valores altos no fibonacci com recursão de cauda).
- [x] Descreve erros em runtime indicando a linha/coluna e o código do trecho problemático.
- [x] Suporta recursões profundas.

## Como utilizar
```
go run . ./examples/fib.json
```

## Como testar
Dependências:
```
cargo install rinha
```
Execução dos testes:
```
go test -v ./interpreter
```
ou 
```
go run . ./examples/my-test.rinha
```
