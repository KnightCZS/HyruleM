# Validação 0.4.0

## Executado neste ambiente

- Testes Go com detector de corrida, incluindo parsers/filtro/sandbox anteriores.
- Servidor TCP real com dois clientes: conexão, IDs/slots únicos, transmissão de pose, renomeação, rejeição de terceiro cliente, desconexão, reutilização de slot e desligamento do servidor.
- Rejeição de JSON inválido, pacote excessivo, estado não finito/fora dos limites e nomes inválidos.
- Ponte de duas fontes de controle simuladas até o servidor, interpolação e projeção do avatar; movimento do primeiro controle aparece nos dados e na geometria da segunda câmera.
- Preservação de campanha, saves separados por cliente, não sobrescrever save ao reconectar, configurações separadas, vínculo para recursos e ambiente sem opções de teste herdadas.
- Acompanhamento separado das árvores de processos: uma sessão não adota o outro cliente.
- Projeção frente/trás/lado, ocultação com câmera antiga e inspeção visual de uma prévia SVG/PNG do avatar procedural.
- Compilação cruzada Windows amd64 do launcher e servidor, sem dependências Go externas.

A prévia geométrica e os controles simulados não são capturas nem execução de BOTW.

## Pendente em Windows com o jogo

O ambiente de desenvolvimento não possui dump do jogo, DLL gerada do proprietário ou GPU/desktop Windows para rodar duas sessões reais. Portanto não foram confirmados: boot simultâneo do runtime, consumo de VRAM, tracking da janela em todas as configurações, calibração/oclusão da imagem e posicionamento visível em Hateno. A CI Windows testa também as junctions e compila os executáveis; ela não possui o jogo.

## Critérios do teste manual

1. Campanha e sandbox originais continuam abrindo e seus arquivos não são alterados pelo teste de rede.
2. Cliente 1 e 2 chegam ao mundo e confirmam sincronização; portas e saves são diferentes.
3. Cada janela mostra somente a representação do outro jogador, com nome correto.
4. Caminhar/correr/pular muda a posição recebida; girar a câmera mantém a projeção na direção esperada, respeitando as limitações documentadas de FOV e latência.
5. Alterar nome aparece no cliente remoto; desconectar remove o avatar; reconectar não restaura o save para a cópia inicial.
6. Minimizar/restaurar e alternar foco não capturam input nem desenham sobre outros aplicativos.
7. Fechar um jogo não encerra nem faz o launcher adotar a outra instância.
8. Repetir entre dois PCs e registrar latência, versão e logs de ambos.
