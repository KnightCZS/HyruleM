# HyruleM — presença nativa experimental

Este build implementa o caminho de criação, posicionamento, animação e remoção de um ator remoto dentro do jogo. **Ainda não é a Gen1 completa solicitada.** Colisão funcional, combate entre Links, coleta sincronizada e autoridade dos monstros precisam de integração e validação em jogo. O servidor Gen1 continua recusando este adaptador como backend completo.

O modelo é montado localmente com os recursos do seu BOTW Switch 1.9.0. O pacote distribuído contém somente nosso código e a atualização do runtime; não contém o jogo, modelos, saves ou `swgame.dll`.

## Instalação do teste

1. Feche os dois jogos. Extraia este pacote em uma pasta própria.
2. No PowerShell, execute `powershell -ExecutionPolicy Bypass -File .\Install-NativePreview.ps1 -RuntimeDir 'C:\caminho\Savage Wind'`, apontando para sua instalação existente do Savage Wind 0.5.3-alpha.5. O instalador guarda backup dos componentes substituídos e preserva jogo, configurações e saves.
3. Abra a configuração do Savage Wind e deixe a biblioteca do jogo ser reconstruída com o gerador atualizado. Não renomeie nem edite o arquivo `.id` para forçar compatibilidade.
4. Abra **Iniciar-Nativo.cmd**. Use o menu multiplayer existente, IP, porta e nomes. O primeiro lançamento gera automaticamente o overlay privado; o segundo reutiliza esse overlay e continua com save e porta de controle próprios.
5. Inicie o servidor de presença existente (protocolo 1, porta padrão 5050) e conecte os dois clientes. Escolha Continuar nos jogos e aguarde o posicionamento em Hateno. O modo nativo não desenha o boneco externo da versão anterior.

**Esta implementação ainda não foi aprovada em um teste real com dois jogos.** Uma confirmação de comando significa apenas que o pedido entrou na fila. A mensagem de ator criado só aparece depois que o jogo confirma a criação e aplica uma pose. Falhas de criação interrompem o teste, sem repetir o spawn indefinidamente.

Fechar a conexão remove apenas o ator administrado pelo HyruleM. Um timeout também remove a presença antiga. Para voltar ao build estável, abra o launcher normalmente, sem `Iniciar-Nativo.cmd`; para restaurar o runtime anterior, com os jogos fechados, recoloque os componentes do backup e permita reconstruir novamente a biblioteca local.

## Validação e limites

- Validados: assinaturas únicas no módulo fornecido; leitura e escrita dos recursos conferidas por parser independente; os 7.944 registros originais do ActorInfo permanecem intactos; testes com memória sintética para fila, referências, remoção, rejeição de replay e preservação dos registradores.
- Pendente: inicialização do DemoNPC com os recursos do Link, aparência/animação em GPU real, transições de mapa, colisões e interação.
- Vida, inventário e regras de alvo já têm testes no servidor autoritativo, mas ainda não estão ligados aos eventos nativos de combate/coleta/IA. O teste usa o protocolo de presença, que não sincroniza esses sistemas.

Código do runtime base: [Savage Wind Recomp](https://github.com/Revivedx/SavageWindRecomp), commit `740b18025fe11ba03c117dbb56668db304b5043e`, licença MIT. Aplicação reproduzível: `python native/apply_runtime.py /caminho/do/source`; reconstruir o executável e `make gen-helper`. Nenhum código gerado do jogo deve ser enviado ao repositório.
