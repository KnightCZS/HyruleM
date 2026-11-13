# Validação — 0.3.0

## Resultado

Os testes do núcleo e dos formatos passam no Linux com detector de condições de corrida. O executável e os testes são compilados para Windows x64. A interface nova e o filtro ainda não foram executados com BOTW real neste ambiente.

O usuário confirmou a versão 0.2 abrindo em Hateno, sem torres ou runas. Essa confirmação é a base preservada; não valida automaticamente o filtro 0.3.

## Verificações do filtro

- Remoção de Main com preservação dos valores e etapas de Mini/Sub.
- Conferência das famílias narrativas e das exceções de gameplay.
- Substituição de metadados de filme/timeline por fluxo válido.
- Um ator e uma ação vinculados; ação de espera de um quadro e término normal.
- Dicionários de pontos de entrada testados por busca, não apenas por leitura sequencial.
- Inclusão de auxiliares internos e de entradas existentes em cópias do fluxo dentro de outros pacotes.
- Geração de pacote para evento que antes só possuía filme.
- Descritor AAMP sem reposicionamento, pausa global ou carregamento de filme narrativo.
- Remoção de EventTag, preservação de NPC e limpeza de ligação para a tag removida.
- Preservação byte a byte dos arquivos originais do RomFS usado nos testes.
- Cache, mudança de política, alterações de origem e regeneração de saída modificada.
- Falha e cancelamento sem publicação parcial ou perda do filtro anterior.
- Overlay aplicado ao sandbox e removido ao construir a execução de campanha/compatibilidade.
- Detecção de opções de lançamento que substituiriam a sessão, sem regravar as opções do usuário.

Esses testes usam uma instalação sintética, não uma cópia do jogo.

## Formatos e verificações independentes

- BYML: valores tipados, strings Unicode, arrays, mapas, números de 32/64 bits e ambos os byte orders; rejeição de estruturas truncadas, referências inválidas e ciclos.
- Yaz0: referências sobrepostas, limites da janela, compressão/descompressão e rejeição de dados corrompidos.
- SARC: substituição com crescimento, alinhamento e preservação dos demais arquivos.
- RSTB: caminhos canônicos, orçamentos dos novos recursos e manutenção de entradas desconhecidas.
- BFEVFL: nomes, dicionários, ações, parâmetros e relocação.
- AAMP: cabeçalho, objetos, tipos, offsets e valores do descritor.
- O recurso BFEVFL gerado foi lido pela biblioteca independente evfl: seis entradas, EventSystemActor, Demo_WaitFrame, Frame=1, IsWaitFinish=true e próximo evento ausente.
- O descritor gerado foi lido pelo parser independente aamp, e o BYML de verificação pelo leitor do conversor upstream.
- Uma rodada de fuzzing de BYML processou aproximadamente 902 mil entradas em três segundos sem falha observada. Isso é um teste de limites do parser, não de gameplay.

As bibliotecas usadas para verificação não fazem parte das dependências do launcher entregue.

## Verificações mantidas

- Saves separados, cópia original preservada, posição, retomada e backup de recriação.
- A criação não concede runas implicitamente.
- Comandos locais com servidor TCP simulado: prontidão, erros, timeout/cancelamento, kit e espera da conclusão do teleporte.
- Argumentos diretos, sem shell; caminhos Unicode e com espaços.
- Configuração persistente e backup da configuração anterior.
- Processo real de teste, execução duplicada e relato de encerramento com erro.
- ZIPs, cancelamento e rejeição de pacote incorreto sem publicar instalação parcial.
- Instalação e reutilização do pacote oficial local do Savage Wind.
- SHA-256 do runtime: `667c4909a8ed72c002e5aa40a00d38720627eebf48b860beb5cd8805421f1285`.
- Compilação PE x64 GUI com recursos, manifesto de DPI e tamanhos das estruturas Win32 conferidos na compilação.

## Aceite dentro do jogo

1. Abrir a atualização com a instalação e o sandbox existentes; não usar Novo + backup.
2. Gerar o filtro e carregar o sandbox por Continuar.
3. Abrir o diário: conferir principais ausentes e secundárias/santuários preservados.
4. Aceitar e avançar uma secundária; testar uma loja, um baú e cozinha.
5. Testar viagem e entrada/saída de santuário.
6. Visitar um gatilho conhecido da história e conferir ausência da sequência narrativa.
7. Salvar, fechar e retomar, mantendo posição e progresso do sandbox.
8. Iniciar a campanha normal e conferir que ela não recebe o overlay.
9. Se houver falha, selecionar o modo de compatibilidade e guardar o diagnóstico de Abrir logs.

Esses passos continuam pendentes para a camada 0.3. NPCs, chefes e todas as regiões não foram declarados validados.
