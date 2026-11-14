# HyruleM 0.5.0-preview.8

A prévia 7 foi confirmada pelo jogador com dois Links visíveis e em movimento. A prévia 8 mantém o callback após a espera das tarefas dos atores e acrescenta horário/clima do anfitrião, equipamento visual e melhorias na reprodução das ações. Os novos visuais ainda precisam de validação em dois jogos reais; os testes automatizados não verificam a GPU.

## Instalar

1. Feche os dois jogos e o launcher. Extraia o **ZIP inteiro**, inclusive `runtime-update`, para uma pasta própria.
2. Execute **Iniciar-Nativo.cmd**. Ele atualiza o runtime configurado com backup; não basta trocar apenas o EXE do launcher.
3. Aguarde a preparação do overlay. Ele é regenerado nesta versão e deriva os prefabs visuais dos arquivos da sua própria instalação. Não contém modelos no download.
4. Use a prévia 8 nos **dois clientes e no servidor**: ABI 5 / protocolo 5. A biblioteca gerada pela prévia 7 pode ser reaproveitada, pois os hooks/headers do gerador não mudaram; versões anteriores à 7 precisam regenerar. Se o runtime solicitar compilação, aguarde; nunca edite arquivos `.id` para contornar a verificação.
5. Hospede na interface/IP do PC e porta escolhida. No mesmo computador, use os botões cliente 1 e cliente 2 e `127.0.0.1`; entre PCs/Radmin use o IP do anfitrião. Entre no mundo aberto e aproxime os jogadores.

## O que mudou

- **Horário e clima:** o primeiro jogador conectado é o anfitrião ambiental. Clientes seguem sua hora, pausa do relógio e clima atual; quem entra depois recebe o estado recente. O servidor escolhe um sucessor quando o anfitrião sai. Epoch/seq descartam amostras antigas e rejeitam alterações de outros clientes.
- **Encerramento:** desconexão, troca de autoridade, carregamento ou amostra vencida restauram as políticas anteriores de clima/relógio, quando o controle está disponível, sem voltar a hora para o início da sessão nem sobrescrever uma política alterada manualmente.
- **Runtime nativo e marionete:** criação, referências, movimento e limpeza continuam na thread do jogo, após a barreira dos jobs. São atores DemoNPC com controladores visuais; não se cria outro singleton Player. As verificações nativas de acesso permanecem ativas.
- **Equipamento:** captura de espada/lança/arma de duas mãos, arco, escudo, cabeça, tronco e pernas. Cada peça remota é um ator visual independente, com transformação relativa ao dono. Cabeça padrão inclui cabelo. O gerador usa `bfres` e `mainModel` do ActorInfo; a instalação 1.9.0 analisada produziu 602 prefabs.
- **Animações:** preserva os slots AS, evita reiniciar continuamente Wait/Move/Walk/Run iguais e conserva o Morph válido do clipe representativo. Ataques forçados com sequência nova continuam sendo reproduzidos.
- **Movimento:** posição, rotação, ações e seis peças viajam em um comando atômico; snapshot local único, intervalo menor de leitura, interpolação de direção pelo caminho curto e suavização por frame. Saúde de monstros só gera comandos quando muda, com limite de quatro aplicações por rodada.

## Limites reais

Equipamento é **aparência**, não cópia de itens: não altera inventário, durabilidade ou vida do outro jogador. Ainda não há sincronização exata de fase/velocidade dos clipes, expressões faciais, tinturas, máscaras especiais de corpo, capas, bainhas ou deformação do arco. Algumas combinações podem sobrepor malha, usar o clipe aproximado ou falhar no carregamento. Uma peça que falha não remove o Link nem provoca tentativas ilimitadas.

O clima atual do anfitrião é aplicado globalmente aos clientes, sem previsão independente por região. Datas, Blood Moon, missões, baús, coleta, árvores, mapa e IA de monstros continuam locais. O servidor **não executa BOTW** e o anfitrião ambiental ainda não é autoridade de toda a simulação.

A convergência de vida dos monstros de mapa permanece experimental: até 64 por leitura, 128 por sessão, identidade por hash/nome/posição original e menor vida conhecida, inclusive zero. Isso não sincroniza movimento, alvo, morte animada nem IA por proximidade. Reiniciar o servidor limpa esse registro.

## Diagnóstico e teste sugerido

Teste caminhada, corrida, salto, rotação e golpes nos dois sentidos. Troque cabeça/tronco/pernas, saque/guarde espada, arco e escudo. Confira se só o dono ganha/perde itens e vida. Feche o primeiro cliente e confira a sucessão do horário/clima no outro. Compare em mundo aberto; estar conectado não comprova renderização correta.

Se houver falha, rode **Diagnosticar.cmd** com ambos os jogos abertos e envie os dois JSONs. Os relatórios `native-preview-diag.8` incluem `time`, `weather`, estado local/equipamento e diagnóstico do ator. `equipment_active` e `equipment_json` mostram quantidade, modelo e erro por peça; `active` informa aceitação nativa, sem comprovar desenho na GPU. `native_ready` continua reservado à Gen1 completa.

O modelo principal conserva uma única tentativa de contingência. **Modelo de contingência ativo · movimento sem animação** indica degradação explícita: não recebe AS do Player. A limpeza retém referências enquanto o jogo nega acesso, sem criar duplicatas.

## Validação desta versão

- Testes C com memória sintética: barreira pós-jobs, referências/gerações, CPU preservada, equipamento isolado, transformações, falhas limitadas, parsing atômico, slots/repetição AS e suavização a 30/60/120 Hz.
- Hook emitido compilado e executado com memória sintética; 16 assinaturas únicas verificadas no módulo 1.9.0 privado.
- Testes Go, detector de condições de corrida e vet: autoridade/sucessão/entrada tardia, rejeição de estado antigo, políticas ambientais, interpolação, orçamento de comandos e limites do protocolo.
- Geração privada e leitura independente dos 602 packs: recursos, máscaras parciais de armadura, controladores Dummy/DemoNPC e arma rígida sem máscaras/animações do Player.
- Compilação Windows, instalador com backup/rollback e abertura direta do launcher verificados pelo CI. Validação gráfica desta prévia depende do teste em jogo.

Pesquisa: [RESEARCH_SYNC.md](RESEARCH_SYNC.md). Runtime [Savage Wind](https://github.com/Revivedx/SavageWindRecomp/tree/740b18025fe11ba03c117dbb56668db304b5043e), MIT. Nenhum código/BNP do Milk Bar foi incorporado. O pacote não contém jogo, modelos, saves nem biblioteca recompilada do jogo.
