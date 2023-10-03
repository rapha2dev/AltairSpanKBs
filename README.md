<img src="./altair.png" width="150" height="150">

## Descrição
Feito em Go e exclusivamente para a rinha de compiladores, Altair Spankabytes é um simples interpretador da linguagem Rinha.

## Características
- Baseado em tree-walking interpreter.
- Feito apenas em Go 1.21 e sua biblioteca padrão.
- Todas as otimizações são genéricas, não importando se é um cálculo de fibonacci, fatorial, etc.
- Interpreta em duas etapas:
    1. Pré-Runtime: faz verificações para memoização, configura a construção de escopos e cria funções específicas para executar cada nó da AST.
    2. Runtime: execução recursiva dos nós e verificações de erros.

## Funcionalidades
- [x] Shadowing
- [x] Memoização automática
- [x] Usa int64 como padrão e, caso necessário, faz conversão automática em runtime para bigint.
- [x] Descreve erros em runtime indicando a linha/coluna e o código do trecho problemático.
- [x] Suporta recursões profundas.

## Desempenho
Intel(R) Core(TM) i5-9600KF CPU @ 3.70GHz

|Exemplo           | Altair           
|:-----------------|:-----------------:|
|fib(46)           |~0.0005 secs       |
|fib(430000)       |~6.1 secs          |
|hanoi(420000)     |~15.0 secs         |
|sum(420000)       |~0.82 secs         |

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
go run . ./examples/fib.rinha time
```
